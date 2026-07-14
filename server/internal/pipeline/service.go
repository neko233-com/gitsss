// Copyright 2026 gitsss Authors. All rights reserved.

package pipeline

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
	runNamespace    = "pipeline-runs"
	leaseDuration   = 2 * time.Minute
	maximumAttempts = 3
	maximumLogBytes = 128 << 10
	maximumLogChunk = 32 << 10
)

var (
	ErrInvalid  = errors.New("invalid pipeline request")
	ErrNotFound = errors.New("pipeline run not found")
	ErrConflict = errors.New("pipeline run state conflict")
	ErrState    = errors.New("pipeline state unavailable")
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Step struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Command          []string `json:"command"`
	WorkingDirectory string   `json:"working_directory"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
}

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
}

type Run struct {
	ID            string    `json:"id"`
	RepositoryID  string    `json:"repository_id"`
	Commit        string    `json:"commit"`
	TemplateID    string    `json:"template_id"`
	Status        Status    `json:"status"`
	RequestedBy   string    `json:"requested_by"`
	RunnerID      string    `json:"runner_id,omitempty"`
	Attempts      int       `json:"attempts"`
	Logs          string    `json:"logs,omitempty"`
	LogsTruncated bool      `json:"logs_truncated"`
	Error         string    `json:"error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	LeaseUntil    time.Time `json:"lease_until,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

type Claim struct {
	Run      Run      `json:"run"`
	Template Template `json:"template"`
}

type Service struct {
	mu    sync.RWMutex
	runs  map[string]Run
	store *state.Store
	audit *audit.Log
}

var templates = map[string]Template{
	"go-verify": {
		ID: "go-verify", Name: "Go 快速验证", Description: "格式、单元测试和静态检查",
		Steps: []Step{
			{ID: "test", Name: "Go tests", Command: []string{"go", "test", "./..."}, WorkingDirectory: ".", TimeoutSeconds: 120},
			{ID: "vet", Name: "Go vet", Command: []string{"go", "vet", "./..."}, WorkingDirectory: ".", TimeoutSeconds: 120},
		},
	},
	"web-verify": {
		ID: "web-verify", Name: "Web 快速验证", Description: "冻结安装、lint、类型、测试和生产构建",
		Steps: []Step{
			{ID: "install", Name: "Frozen install", Command: []string{"pnpm", "install", "--frozen-lockfile"}, WorkingDirectory: ".", TimeoutSeconds: 120},
			{ID: "lint", Name: "Oxlint", Command: []string{"pnpm", "exec", "oxlint", "src"}, WorkingDirectory: ".", TimeoutSeconds: 120},
			{ID: "types", Name: "TypeScript", Command: []string{"pnpm", "exec", "tsc", "-b", "--noEmit"}, WorkingDirectory: ".", TimeoutSeconds: 120},
			{ID: "test", Name: "Vitest", Command: []string{"pnpm", "exec", "vitest", "run"}, WorkingDirectory: ".", TimeoutSeconds: 120},
			{ID: "build", Name: "Vite build", Command: []string{"pnpm", "exec", "vite", "build"}, WorkingDirectory: ".", TimeoutSeconds: 120},
		},
	},
	"gitsss-verify": {
		ID: "gitsss-verify", Name: "gitsss 整体验证", Description: "服务端和 React 客户端的本机发布门禁",
		Steps: []Step{
			{ID: "go-test", Name: "Server tests", Command: []string{"go", "test", "./..."}, WorkingDirectory: "server", TimeoutSeconds: 120},
			{ID: "go-vet", Name: "Server vet", Command: []string{"go", "vet", "./..."}, WorkingDirectory: "server", TimeoutSeconds: 120},
			{ID: "web-install", Name: "Web frozen install", Command: []string{"pnpm", "install", "--frozen-lockfile"}, WorkingDirectory: "web", TimeoutSeconds: 120},
			{ID: "web-lint", Name: "Web lint", Command: []string{"pnpm", "exec", "oxlint", "src"}, WorkingDirectory: "web", TimeoutSeconds: 120},
			{ID: "web-types", Name: "Web types", Command: []string{"pnpm", "exec", "tsc", "-b", "--noEmit"}, WorkingDirectory: "web", TimeoutSeconds: 120},
			{ID: "web-test", Name: "Web tests", Command: []string{"pnpm", "exec", "vitest", "run"}, WorkingDirectory: "web", TimeoutSeconds: 120},
			{ID: "web-build", Name: "Web build", Command: []string{"pnpm", "exec", "vite", "build"}, WorkingDirectory: "web", TimeoutSeconds: 120},
		},
	},
}

func NewPersistentService(log *audit.Log, store *state.Store) (*Service, error) {
	if log == nil || store == nil {
		return nil, ErrState
	}
	service := &Service{runs: make(map[string]Run), store: store, audit: log}
	documents, err := store.List(runNamespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	for _, document := range documents {
		var run Run
		if err := json.Unmarshal(document.Payload, &run); err != nil || document.ID != run.ID || !validRun(run) {
			return nil, fmt.Errorf("%w: corrupt run %q", ErrState, document.ID)
		}
		service.runs[run.ID] = run
	}
	return service, nil
}

func Templates() []Template {
	result := make([]Template, 0, len(templates))
	for _, template := range templates {
		result = append(result, cloneTemplate(template))
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func TemplateByID(id string) (Template, bool) {
	template, ok := templates[id]
	if !ok {
		return Template{}, false
	}
	return cloneTemplate(template), true
}

func (s *Service) Create(actor, requestID, repositoryID, commit, templateID string, now time.Time) (Run, error) {
	if !validText(actor, 128, true) || !validText(requestID, 128, true) || !validRepositoryID(repositoryID) ||
		!validCommit(commit) || now.IsZero() {
		return Run{}, ErrInvalid
	}
	if _, ok := templates[templateID]; !ok {
		return Run{}, ErrInvalid
	}
	run := Run{
		ID: newID(), RepositoryID: repositoryID, Commit: commit, TemplateID: templateID,
		Status: StatusQueued, RequestedBy: actor, CreatedAt: now.UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persistWithAudit(Run{}, false, run, audit.Entry{
		Actor: actor, Action: "pipeline.create", Target: "pipeline:" + run.ID,
		RequestID: requestID, Result: string(run.Status), At: now.UTC(),
	}); err != nil {
		return Run{}, err
	}
	s.runs[run.ID] = run
	return cloneRun(run), nil
}

func (s *Service) Claim(actor, requestID, runnerID string, now time.Time) (Claim, bool, error) {
	if !validText(actor, 128, true) || !validText(requestID, 128, true) || !validText(runnerID, 128, true) || now.IsZero() {
		return Claim{}, false, ErrInvalid
	}
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	ordered := s.orderedRunsLocked()
	for _, candidate := range ordered {
		if candidate.Status == StatusRunning && !candidate.LeaseUntil.After(now) && candidate.Attempts >= maximumAttempts {
			failed := candidate
			failed.Status = StatusFailed
			failed.Error = "runner lease expired after three attempts"
			failed.CompletedAt = now
			failed.LeaseUntil = time.Time{}
			if err := s.persistWithAudit(candidate, true, failed, audit.Entry{
				Actor: actor, Action: "pipeline.expire", Target: "pipeline:" + failed.ID,
				RequestID: requestID, Result: string(failed.Status), At: now,
			}); err != nil {
				return Claim{}, false, err
			}
			s.runs[failed.ID] = failed
			continue
		}
		eligible := candidate.Status == StatusQueued || candidate.Status == StatusRunning && !candidate.LeaseUntil.After(now)
		if !eligible {
			continue
		}
		claimed := candidate
		claimed.Status = StatusRunning
		claimed.RunnerID = runnerID
		claimed.Attempts++
		if claimed.StartedAt.IsZero() {
			claimed.StartedAt = now
		}
		claimed.LeaseUntil = now.Add(leaseDuration)
		if err := s.persistWithAudit(candidate, true, claimed, audit.Entry{
			Actor: actor, Action: "pipeline.claim", Target: "pipeline:" + claimed.ID,
			RequestID: requestID, Result: runnerID, At: now,
		}); err != nil {
			return Claim{}, false, err
		}
		s.runs[claimed.ID] = claimed
		return Claim{Run: cloneRun(claimed), Template: cloneTemplate(templates[claimed.TemplateID])}, true, nil
	}
	return Claim{}, false, nil
}

func (s *Service) Heartbeat(runID, runnerID, logChunk string, now time.Time) (Run, error) {
	if !validText(runID, 128, true) || !validText(runnerID, 128, true) || now.IsZero() || len(logChunk) > maximumLogChunk || strings.ContainsRune(logChunk, 0) {
		return Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.runs[runID]
	if !ok {
		return Run{}, ErrNotFound
	}
	if current.Status != StatusRunning || current.RunnerID != runnerID {
		return Run{}, ErrConflict
	}
	updated := current
	updated.LeaseUntil = now.UTC().Add(leaseDuration)
	updated.Logs, updated.LogsTruncated = appendLog(updated.Logs, updated.LogsTruncated, logChunk)
	if err := s.persist(updated); err != nil {
		return Run{}, err
	}
	s.runs[runID] = updated
	return cloneRun(updated), nil
}

func (s *Service) Complete(actor, requestID, runID, runnerID string, succeeded bool, failure string, now time.Time) (Run, error) {
	if !validText(actor, 128, true) || !validText(requestID, 128, true) || !validText(runID, 128, true) ||
		!validText(runnerID, 128, true) || !validText(failure, 1024, false) || now.IsZero() {
		return Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.runs[runID]
	if !ok {
		return Run{}, ErrNotFound
	}
	if current.Status == StatusSucceeded || current.Status == StatusFailed {
		if current.RunnerID != runnerID {
			return Run{}, ErrConflict
		}
		return cloneRun(current), nil
	}
	if current.Status != StatusRunning || current.RunnerID != runnerID {
		return Run{}, ErrConflict
	}
	completed := current
	completed.Status = StatusFailed
	if succeeded {
		completed.Status = StatusSucceeded
		failure = ""
	}
	completed.Error = strings.TrimSpace(failure)
	completed.CompletedAt = now.UTC()
	completed.LeaseUntil = time.Time{}
	if err := s.persistWithAudit(current, true, completed, audit.Entry{
		Actor: actor, Action: "pipeline.complete", Target: "pipeline:" + completed.ID,
		RequestID: requestID, Result: string(completed.Status), At: now.UTC(),
	}); err != nil {
		return Run{}, err
	}
	s.runs[runID] = completed
	return cloneRun(completed), nil
}

func (s *Service) Runs() []Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ordered := s.orderedRunsLocked()
	result := make([]Run, len(ordered))
	for index, run := range ordered {
		result[index] = cloneRun(run)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].CreatedAt.After(result[right].CreatedAt) })
	return result
}

func (s *Service) orderedRunsLocked() []Run {
	result := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		result = append(result, run)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].CreatedAt.Equal(result[right].CreatedAt) {
			return result[left].ID < result[right].ID
		}
		return result[left].CreatedAt.Before(result[right].CreatedAt)
	})
	return result
}

func (s *Service) persistWithAudit(previous Run, existed bool, next Run, entry audit.Entry) error {
	if err := s.persist(next); err != nil {
		return err
	}
	if err := s.audit.Append(entry); err != nil {
		var rollbackErr error
		if existed {
			rollbackErr = s.persist(previous)
		} else {
			rollbackErr = s.store.DeleteMany(runNamespace, []string{next.ID})
		}
		return errors.Join(err, rollbackErr)
	}
	return nil
}

func (s *Service) persist(run Run) error {
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	if err := s.store.Put(runNamespace, run.ID, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func appendLog(current string, truncated bool, chunk string) (string, bool) {
	combined := redactLog(current + chunk)
	if len(combined) <= maximumLogBytes {
		return combined, truncated
	}
	return combined[:maximumLogBytes], true
}

func redactLog(value string) string {
	fields := strings.FieldsFunc(value, func(character rune) bool {
		return character == ' ' || character == '\t' || character == '\r' || character == '\n'
	})
	for _, field := range fields {
		if strings.HasPrefix(field, "gitsss_") && len(field) >= 27 {
			value = strings.ReplaceAll(value, field, "[REDACTED_TOKEN]")
		}
	}
	return value
}

func validRun(run Run) bool {
	if !validRunID(run.ID) || !validRepositoryID(run.RepositoryID) || !validCommit(run.Commit) ||
		!validText(run.RequestedBy, 128, true) || !validText(run.RunnerID, 128, false) || !validText(run.Error, 1024, false) ||
		len(run.Logs) > maximumLogBytes || strings.ContainsRune(run.Logs, 0) || run.CreatedAt.IsZero() || run.Attempts < 0 || run.Attempts > maximumAttempts {
		return false
	}
	if _, ok := templates[run.TemplateID]; !ok {
		return false
	}
	switch run.Status {
	case StatusQueued:
		return run.RunnerID == "" && run.Attempts == 0 && run.StartedAt.IsZero() && run.LeaseUntil.IsZero() && run.CompletedAt.IsZero()
	case StatusRunning:
		return run.RunnerID != "" && run.Attempts >= 1 && !run.StartedAt.IsZero() && !run.LeaseUntil.IsZero() && run.CompletedAt.IsZero()
	case StatusSucceeded, StatusFailed:
		return run.Attempts >= 1 && !run.StartedAt.IsZero() && run.LeaseUntil.IsZero() && !run.CompletedAt.IsZero()
	default:
		return false
	}
}

func validRunID(id string) bool {
	decoded, err := hex.DecodeString(strings.TrimPrefix(id, "pipeline-"))
	return strings.HasPrefix(id, "pipeline-") && err == nil && len(decoded) == 12
}

func validRepositoryID(id string) bool {
	if id == "" || len(id) > 64 || id[0] == '-' || id[len(id)-1] == '-' {
		return false
	}
	for _, character := range id {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

func validText(value string, maximum int, required bool) bool {
	if required && strings.TrimSpace(value) == "" {
		return false
	}
	return len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}

func cloneRun(run Run) Run {
	return run
}

func cloneTemplate(template Template) Template {
	template.Steps = append([]Step(nil), template.Steps...)
	for index := range template.Steps {
		template.Steps[index].Command = append([]string(nil), template.Steps[index].Command...)
	}
	return template
}

func newID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("pipeline-%024x", uint64(time.Now().UnixNano()))
	}
	return "pipeline-" + hex.EncodeToString(value)
}
