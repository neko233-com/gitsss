// Copyright 2026 gitsss Authors. All rights reserved.

package agentjob

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

const (
	maxInputBytes  = 32 << 10
	maxResultBytes = 256 << 10
	leaseDuration  = 5 * time.Minute
	stateNamespace = "agent-jobs"
)

var (
	ErrInvalidJob = errors.New("invalid Agent job")
	ErrNotFound   = errors.New("Agent job not found")
	ErrWrongAgent = errors.New("Agent job belongs to another Agent")
)

type Kind string
type Status string

const (
	KindAssessment    Kind = "takeover.assess"
	KindBackupExport  Kind = "backup.export"
	KindBackupAudit   Kind = "backup.audit-restore"
	KindBackupRestore Kind = "backup.restore-isolated"
	KindBackupVerify  Kind = "backup.verify"
	KindShadowAudit   Kind = "takeover.shadow-audit"
	KindFreezeCheck   Kind = "takeover.freeze-check"

	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID         string          `json:"id"`
	Kind       Kind            `json:"kind"`
	Status     Status          `json:"status"`
	Input      json.RawMessage `json:"input"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
	ClaimedBy  string          `json:"claimed_by,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	LeaseUntil time.Time       `json:"lease_until,omitempty"`
}

type Queue struct {
	mu    sync.Mutex
	jobs  map[string]*Job
	order []string
	audit *audit.Log
	store *state.Store
}

func NewQueue(auditLog *audit.Log) *Queue {
	return &Queue{jobs: map[string]*Job{}, audit: auditLog}
}

func NewPersistentQueue(auditLog *audit.Log, store *state.Store) (*Queue, error) {
	if store == nil {
		return nil, errors.New("Agent job state store is required")
	}
	documents, err := store.List(stateNamespace)
	if err != nil {
		return nil, err
	}
	queue := &Queue{jobs: make(map[string]*Job, len(documents)), audit: auditLog, store: store}
	loaded := make([]Job, 0, len(documents))
	for _, document := range documents {
		var job Job
		if err := json.Unmarshal(document.Payload, &job); err != nil || document.ID != job.ID || !validPersistedJob(job) {
			return nil, fmt.Errorf("%w: corrupt persisted Agent job %q", ErrInvalidJob, document.ID)
		}
		loaded = append(loaded, job)
	}
	sort.Slice(loaded, func(left, right int) bool {
		if loaded[left].CreatedAt.Equal(loaded[right].CreatedAt) {
			return loaded[left].ID < loaded[right].ID
		}
		return loaded[left].CreatedAt.Before(loaded[right].CreatedAt)
	})
	for index := range loaded {
		job := loaded[index]
		queue.jobs[job.ID] = &job
		queue.order = append(queue.order, job.ID)
	}
	return queue, nil
}

func (queue *Queue) Create(actor, requestID string, kind Kind, input json.RawMessage, now time.Time) (Job, error) {
	if !validKind(kind) || len(input) == 0 || len(input) > maxInputBytes || !json.Valid(input) {
		return Job{}, ErrInvalidJob
	}
	job := Job{
		ID: newID(), Kind: kind, Status: StatusQueued, Input: append(json.RawMessage(nil), input...),
		CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	queue.mu.Lock()
	if err := queue.persist(job); err != nil {
		queue.mu.Unlock()
		return Job{}, err
	}
	queue.jobs[job.ID] = &job
	queue.order = append(queue.order, job.ID)
	queue.mu.Unlock()
	if err := queue.record(actor, "agent.job.create", job.ID, requestID, "queued", now); err != nil {
		return Job{}, err
	}
	return clone(job), nil
}

func (queue *Queue) Claim(agentID, requestID string, now time.Time) (Job, bool, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || len(agentID) > 128 {
		return Job{}, false, ErrInvalidJob
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	for _, id := range queue.order {
		job := queue.jobs[id]
		candidate := clone(*job)
		if job.Status == StatusRunning && !job.LeaseUntil.After(now) {
			candidate.Status = StatusQueued
			candidate.ClaimedBy = ""
			candidate.LeaseUntil = time.Time{}
		}
		if candidate.Status != StatusQueued {
			continue
		}
		candidate.Status = StatusRunning
		candidate.ClaimedBy = agentID
		candidate.UpdatedAt = now.UTC()
		candidate.LeaseUntil = now.UTC().Add(leaseDuration)
		if err := queue.persist(candidate); err != nil {
			return Job{}, false, err
		}
		*job = candidate
		if err := queue.record(agentID, "agent.job.claim", candidate.ID, requestID, "running", now); err != nil {
			return Job{}, false, err
		}
		return clone(candidate), true, nil
	}
	return Job{}, false, nil
}

func (queue *Queue) Complete(agentID, requestID, jobID string, result json.RawMessage, failure string, now time.Time) (Job, error) {
	if len(result) > maxResultBytes || (len(result) > 0 && !json.Valid(result)) || len(failure) > 1024 || (len(result) == 0) == (strings.TrimSpace(failure) == "") {
		return Job{}, ErrInvalidJob
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	job := queue.jobs[jobID]
	if job == nil {
		return Job{}, ErrNotFound
	}
	if job.ClaimedBy != agentID {
		return Job{}, ErrWrongAgent
	}
	if job.Status == StatusSucceeded || job.Status == StatusFailed {
		return clone(*job), nil
	}
	if job.Status != StatusRunning {
		return Job{}, ErrInvalidJob
	}
	candidate := clone(*job)
	candidate.Result = append(json.RawMessage(nil), result...)
	candidate.Error = strings.TrimSpace(failure)
	candidate.UpdatedAt = now.UTC()
	candidate.LeaseUntil = time.Time{}
	if candidate.Error == "" {
		candidate.Status = StatusSucceeded
	} else {
		candidate.Status = StatusFailed
	}
	if err := queue.persist(candidate); err != nil {
		return Job{}, err
	}
	*job = candidate
	if err := queue.record(agentID, "agent.job.complete", candidate.ID, requestID, string(candidate.Status), now); err != nil {
		return Job{}, err
	}
	return clone(candidate), nil
}

func (queue *Queue) Jobs() []Job {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	jobs := make([]Job, 0, len(queue.order))
	for _, id := range queue.order {
		jobs = append(jobs, clone(*queue.jobs[id]))
	}
	sort.SliceStable(jobs, func(left, right int) bool { return jobs[left].CreatedAt.After(jobs[right].CreatedAt) })
	return jobs
}

func (queue *Queue) record(actor, action, target, requestID, result string, now time.Time) error {
	if queue.audit != nil {
		return queue.audit.Append(audit.Entry{Actor: actor, Action: action, Target: target, RequestID: requestID, Result: result, At: now.UTC()})
	}
	return nil
}

func validKind(kind Kind) bool {
	return kind == KindAssessment || kind == KindBackupExport || kind == KindBackupAudit || kind == KindBackupRestore || kind == KindBackupVerify || kind == KindShadowAudit || kind == KindFreezeCheck
}

func validPersistedJob(job Job) bool {
	id, err := hex.DecodeString(strings.TrimPrefix(job.ID, "job-"))
	if !strings.HasPrefix(job.ID, "job-") || err != nil || len(id) != 12 || !validKind(job.Kind) || job.CreatedAt.IsZero() || job.UpdatedAt.Before(job.CreatedAt) {
		return false
	}
	if len(job.Input) == 0 || len(job.Input) > maxInputBytes || !json.Valid(job.Input) || len(job.Result) > maxResultBytes || (len(job.Result) > 0 && !json.Valid(job.Result)) || len(job.Error) > 1024 {
		return false
	}
	switch job.Status {
	case StatusQueued:
		return len(job.Result) == 0 && job.Error == "" && job.ClaimedBy == "" && job.LeaseUntil.IsZero()
	case StatusRunning:
		return len(job.Result) == 0 && job.Error == "" && strings.TrimSpace(job.ClaimedBy) != "" && !job.LeaseUntil.IsZero()
	case StatusSucceeded:
		return len(job.Result) > 0 && job.Error == "" && job.LeaseUntil.IsZero()
	case StatusFailed:
		return len(job.Result) == 0 && strings.TrimSpace(job.Error) != "" && job.LeaseUntil.IsZero()
	default:
		return false
	}
}

func (queue *Queue) persist(job Job) error {
	if queue.store == nil {
		return nil
	}
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return queue.store.Put(stateNamespace, job.ID, payload)
}

func clone(job Job) Job {
	job.Input = append(json.RawMessage(nil), job.Input...)
	job.Result = append(json.RawMessage(nil), job.Result...)
	return job
}

func newID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("job-%024x", uint64(time.Now().UnixNano()))
	}
	return "job-" + hex.EncodeToString(value)
}
