// Copyright 2026 gitsss Authors. All rights reserved.

package runner

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalidReport = errors.New("runner report requires an id and platform")

type Status string

const (
	StatusProvisioning Status = "provisioning"
	StatusReady        Status = "ready"
	StatusOffline      Status = "offline"
)

type Action string

const (
	ActionInstall Action = "install"
	ActionUpdate  Action = "update"
	ActionVerify  Action = "verify"
)

type Runner struct {
	ID        string    `json:"id"`
	Platform  string    `json:"platform"`
	Version   string    `json:"version"`
	Status    Status    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Report struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
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
}

func NewService(targetVersion string) *Service {
	if strings.TrimSpace(targetVersion) == "" {
		panic("runner target version must be configured")
	}
	return &Service{targetVersion: targetVersion, runners: make(map[string]Runner)}
}

func (s *Service) Reconcile(report Report, now time.Time) (Reconciliation, error) {
	if strings.TrimSpace(report.ID) == "" || strings.TrimSpace(report.Platform) == "" {
		return Reconciliation{}, ErrInvalidReport
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	runner, exists := s.runners[report.ID]
	if !exists {
		runner = Runner{ID: report.ID, Platform: report.Platform, Version: s.targetVersion, Status: StatusProvisioning, UpdatedAt: now}
		s.runners[runner.ID] = runner
		return Reconciliation{Action: ActionInstall, Runner: runner, Message: "install and register the bundled runner"}, nil
	}

	runner.Platform = report.Platform
	runner.UpdatedAt = now
	if report.Version != s.targetVersion {
		runner.Version = s.targetVersion
		runner.Status = StatusProvisioning
		s.runners[runner.ID] = runner
		return Reconciliation{Action: ActionUpdate, Runner: runner, Message: "update the bundled runner before accepting jobs"}, nil
	}

	runner.Version = report.Version
	runner.Status = StatusReady
	s.runners[runner.ID] = runner
	return Reconciliation{Action: ActionVerify, Runner: runner, Message: "runner is registered and ready"}, nil
}

func (s *Service) Runners() []Runner {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runners := make([]Runner, 0, len(s.runners))
	for _, runner := range s.runners {
		runners = append(runners, runner)
	}
	sort.Slice(runners, func(i, j int) bool { return runners[i].ID < runners[j].ID })
	return runners
}
