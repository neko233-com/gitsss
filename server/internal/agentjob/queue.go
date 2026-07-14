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
)

const (
	maxInputBytes  = 32 << 10
	maxResultBytes = 256 << 10
	leaseDuration  = 5 * time.Minute
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
	KindBackupRestore Kind = "backup.restore-isolated"
	KindBackupVerify  Kind = "backup.verify"

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
}

func NewQueue(auditLog *audit.Log) *Queue {
	return &Queue{jobs: map[string]*Job{}, audit: auditLog}
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
	queue.jobs[job.ID] = &job
	queue.order = append(queue.order, job.ID)
	queue.mu.Unlock()
	queue.record(actor, "agent.job.create", job.ID, requestID, "queued", now)
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
		if job.Status == StatusRunning && !job.LeaseUntil.After(now) {
			job.Status = StatusQueued
			job.ClaimedBy = ""
			job.LeaseUntil = time.Time{}
		}
		if job.Status != StatusQueued {
			continue
		}
		job.Status = StatusRunning
		job.ClaimedBy = agentID
		job.UpdatedAt = now.UTC()
		job.LeaseUntil = now.UTC().Add(leaseDuration)
		queue.record(agentID, "agent.job.claim", job.ID, requestID, "running", now)
		return clone(*job), true, nil
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
	job.Result = append(json.RawMessage(nil), result...)
	job.Error = strings.TrimSpace(failure)
	job.UpdatedAt = now.UTC()
	job.LeaseUntil = time.Time{}
	if job.Error == "" {
		job.Status = StatusSucceeded
	} else {
		job.Status = StatusFailed
	}
	queue.record(agentID, "agent.job.complete", job.ID, requestID, string(job.Status), now)
	return clone(*job), nil
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

func (queue *Queue) record(actor, action, target, requestID, result string, now time.Time) {
	if queue.audit != nil {
		queue.audit.Append(audit.Entry{Actor: actor, Action: action, Target: target, RequestID: requestID, Result: result, At: now.UTC()})
	}
}

func validKind(kind Kind) bool {
	return kind == KindAssessment || kind == KindBackupExport || kind == KindBackupRestore || kind == KindBackupVerify
}

func clone(job Job) Job {
	job.Input = append(json.RawMessage(nil), job.Input...)
	job.Result = append(json.RawMessage(nil), job.Result...)
	return job
}

func newID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	return "job-" + hex.EncodeToString(value)
}
