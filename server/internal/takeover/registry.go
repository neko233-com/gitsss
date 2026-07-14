// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"crypto/sha256"
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

const registryStateNamespace = "takeover-reports"

type Registry struct {
	mu      sync.RWMutex
	reports map[string]Report
	audit   *audit.Log
	store   *state.Store
}

func NewRegistry(log *audit.Log) *Registry {
	return &Registry{reports: make(map[string]Report), audit: log}
}

func NewPersistentRegistry(log *audit.Log, store *state.Store) (*Registry, error) {
	if store == nil {
		return nil, errors.New("takeover state store is required")
	}
	documents, err := store.List(registryStateNamespace)
	if err != nil {
		return nil, err
	}
	registry := &Registry{reports: make(map[string]Report, len(documents)), audit: log, store: store}
	for _, document := range documents {
		var report Report
		if err := json.Unmarshal(document.Payload, &report); err != nil || document.ID != report.ID || !validReport(report) {
			return nil, fmt.Errorf("invalid persisted takeover assessment %q", document.ID)
		}
		registry.reports[report.ID] = report
	}
	return registry, nil
}

func (r *Registry) Submit(actor, requestID string, report Report, now time.Time) (Report, error) {
	if !validReport(report) {
		return Report{}, errors.New("invalid takeover assessment")
	}
	r.mu.Lock()
	if r.store != nil {
		payload, err := json.Marshal(report)
		if err != nil {
			r.mu.Unlock()
			return Report{}, err
		}
		if err := r.store.Put(registryStateNamespace, report.ID, payload); err != nil {
			r.mu.Unlock()
			return Report{}, err
		}
	}
	r.reports[report.ID] = report
	r.mu.Unlock()
	if r.audit != nil {
		if err := r.audit.Append(audit.Entry{Actor: actor, Action: "takeover.assessment.submit", Target: report.ID, RequestID: requestID, At: now.UTC()}); err != nil {
			return Report{}, err
		}
	}
	return report, nil
}

func validReport(report Report) bool {
	id, idError := hex.DecodeString(strings.TrimPrefix(report.ID, "assessment-"))
	digest, digestError := hex.DecodeString(strings.TrimPrefix(report.ConfigDigest, "sha256:"))
	agentID := strings.TrimSpace(report.AgentID)
	if !strings.HasPrefix(report.ID, "assessment-") || idError != nil || len(id) != 8 || !strings.HasPrefix(report.ConfigDigest, "sha256:") || digestError != nil || len(digest) != sha256.Size {
		return false
	}
	if agentID == "" || len(agentID) > 128 || strings.ContainsAny(agentID, "\r\n\x00") || len(report.ConfigPath) > 4096 {
		return false
	}
	return len(report.Assets) <= 32 && len(report.Findings) <= 128
}

func (r *Registry) Reports() []Report {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reports := make([]Report, 0, len(r.reports))
	for _, report := range r.reports {
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].AssessedAt.After(reports[j].AssessedAt) })
	return reports
}
