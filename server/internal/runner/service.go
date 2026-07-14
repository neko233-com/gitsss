// Copyright 2026 gitsss Authors. All rights reserved.

package runner

import (
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
	runnerStateNamespace = "runners"
	defaultOfflineAfter  = 2 * time.Minute
	minimumDiskFreeBytes = 5 << 30
)

var (
	ErrInvalidReport = errors.New("runner report is invalid")
	ErrState         = errors.New("runner state unavailable")
)

type Status string

const (
	StatusProvisioning Status = "provisioning"
	StatusReady        Status = "ready"
	StatusDegraded     Status = "degraded"
	StatusPaused       Status = "paused"
	StatusOffline      Status = "offline"
)

type Action string

const (
	ActionInstall Action = "install"
	ActionUpdate  Action = "update"
	ActionRepair  Action = "repair"
	ActionPause   Action = "pause"
	ActionVerify  Action = "verify"
)

type Runner struct {
	ID            string    `json:"id"`
	Platform      string    `json:"platform"`
	Version       string    `json:"version"`
	Status        Status    `json:"status"`
	Registered    bool      `json:"registered"`
	Healthy       bool      `json:"healthy"`
	LastError     string    `json:"last_error,omitempty"`
	DiskFreeBytes int64     `json:"disk_free_bytes,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Report struct {
	ID            string `json:"id"`
	Platform      string `json:"platform"`
	Version       string `json:"version"`
	Registered    bool   `json:"registered"`
	Healthy       *bool  `json:"healthy,omitempty"`
	Error         string `json:"error,omitempty"`
	DiskFreeBytes int64  `json:"disk_free_bytes,omitempty"`
}

type Reconciliation struct {
	Action  Action `json:"action"`
	Runner  Runner `json:"runner"`
	Message string `json:"message"`
}

type Service struct {
	mu            sync.RWMutex
	targetVersion string
	runners       map[string]Runner
	store         *state.Store
	audit         *audit.Log
	offlineAfter  time.Duration
}

func NewService(targetVersion string) *Service {
	service, err := newService(targetVersion)
	if err != nil {
		panic(err)
	}
	return service
}

func NewPersistentService(targetVersion string, log *audit.Log, store *state.Store) (*Service, error) {
	if log == nil || store == nil {
		return nil, ErrState
	}
	service, err := newService(targetVersion)
	if err != nil {
		return nil, err
	}
	service.audit = log
	service.store = store
	documents, err := store.List(runnerStateNamespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	for _, document := range documents {
		var runner Runner
		if err := json.Unmarshal(document.Payload, &runner); err != nil || document.ID != runner.ID || !validRunner(runner) {
			return nil, fmt.Errorf("%w: corrupt runner %q", ErrState, document.ID)
		}
		service.runners[runner.ID] = runner
	}
	return service, nil
}

func newService(targetVersion string) (*Service, error) {
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" || len(targetVersion) > 64 || strings.ContainsAny(targetVersion, "\r\n\x00") {
		return nil, errors.New("runner target version must be configured")
	}
	return &Service{
		targetVersion: targetVersion, runners: make(map[string]Runner), offlineAfter: defaultOfflineAfter,
	}, nil
}

func (s *Service) Reconcile(report Report, now time.Time) (Reconciliation, error) {
	return s.ReconcileAs(report.ID, fmt.Sprintf("runner-%d", now.UnixNano()), report, now)
}

func (s *Service) ReconcileAs(actor, requestID string, report Report, now time.Time) (Reconciliation, error) {
	if !validText(actor, 128, true) || !validText(requestID, 128, true) || !validReport(report) || now.IsZero() {
		return Reconciliation{}, ErrInvalidReport
	}
	now = now.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.runners[report.ID]
	registered := report.Registered || report.Version != ""
	healthy := report.Error == ""
	if report.Healthy != nil {
		healthy = *report.Healthy
	}
	runner := Runner{
		ID: report.ID, Platform: report.Platform, Version: report.Version, Registered: registered,
		Healthy: healthy, LastError: report.Error, DiskFreeBytes: report.DiskFreeBytes, UpdatedAt: now,
	}
	reconciliation := Reconciliation{Runner: runner}
	switch {
	case !exists || !registered:
		runner.Version = s.targetVersion
		runner.Status = StatusProvisioning
		reconciliation.Action = ActionInstall
		reconciliation.Message = "install and register the bundled runner"
	case report.Version != s.targetVersion:
		runner.Version = s.targetVersion
		runner.Status = StatusProvisioning
		reconciliation.Action = ActionUpdate
		reconciliation.Message = "update the bundled runner before accepting jobs"
	case report.DiskFreeBytes > 0 && report.DiskFreeBytes < minimumDiskFreeBytes:
		runner.Status = StatusPaused
		runner.Healthy = false
		reconciliation.Action = ActionPause
		reconciliation.Message = "pause new jobs until the runner work volume has at least 5 GiB free"
	case !healthy:
		runner.Status = StatusDegraded
		reconciliation.Action = ActionRepair
		reconciliation.Message = "repair the bundled runner and repeat its health check"
	default:
		runner.Status = StatusReady
		reconciliation.Action = ActionVerify
		reconciliation.Message = "runner is registered and ready"
	}
	reconciliation.Runner = runner
	if err := s.persistReconciliation(actor, requestID, current, exists, reconciliation, now); err != nil {
		return Reconciliation{}, err
	}
	s.runners[runner.ID] = runner
	return reconciliation, nil
}

func (s *Service) Runners() []Runner {
	return s.RunnersAt(time.Now().UTC())
}

func (s *Service) RunnersAt(now time.Time) []Runner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runners := make([]Runner, 0, len(s.runners))
	for _, runner := range s.runners {
		if now.Sub(runner.UpdatedAt) > s.offlineAfter {
			runner.Status = StatusOffline
			runner.Healthy = false
		}
		runners = append(runners, runner)
	}
	sort.Slice(runners, func(left, right int) bool { return runners[left].ID < runners[right].ID })
	return runners
}

func (s *Service) persistReconciliation(actor, requestID string, previous Runner, existed bool, reconciliation Reconciliation, now time.Time) error {
	if s.store == nil {
		return nil
	}
	payload, err := json.Marshal(reconciliation.Runner)
	if err != nil {
		return err
	}
	if err := s.store.Put(runnerStateNamespace, reconciliation.Runner.ID, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	auditErr := s.audit.Append(audit.Entry{
		Actor: actor, Action: "runner.reconcile." + string(reconciliation.Action),
		Target: "runner:" + reconciliation.Runner.ID, RequestID: requestID,
		Result: string(reconciliation.Runner.Status), At: now,
	})
	if auditErr == nil {
		return nil
	}
	var rollbackErr error
	if existed {
		previousPayload, marshalErr := json.Marshal(previous)
		if marshalErr != nil {
			rollbackErr = marshalErr
		} else {
			rollbackErr = s.store.Put(runnerStateNamespace, previous.ID, previousPayload)
		}
	} else {
		rollbackErr = s.store.DeleteMany(runnerStateNamespace, []string{reconciliation.Runner.ID})
	}
	return errors.Join(auditErr, rollbackErr)
}

func validReport(report Report) bool {
	return validText(report.ID, 128, true) && validPlatform(report.Platform) &&
		validText(report.Version, 64, false) && validText(report.Error, 512, false) && report.DiskFreeBytes >= 0
}

func validRunner(runner Runner) bool {
	return validText(runner.ID, 128, true) && validPlatform(runner.Platform) && validText(runner.Version, 64, true) &&
		validText(runner.LastError, 512, false) && runner.DiskFreeBytes >= 0 && !runner.UpdatedAt.IsZero() &&
		(runner.Status == StatusProvisioning || runner.Status == StatusReady || runner.Status == StatusDegraded || runner.Status == StatusPaused)
}

func validPlatform(platform string) bool {
	switch platform {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64":
		return true
	default:
		return false
	}
}

func validText(value string, maximum int, required bool) bool {
	if required && strings.TrimSpace(value) == "" {
		return false
	}
	return len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}
