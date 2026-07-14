// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

type Registry struct {
	mu      sync.RWMutex
	reports map[string]Report
	audit   *audit.Log
}

func NewRegistry(log *audit.Log) *Registry {
	return &Registry{reports: make(map[string]Report), audit: log}
}

func (r *Registry) Submit(actor, requestID string, report Report, now time.Time) (Report, error) {
	if !validReport(report) {
		return Report{}, errors.New("invalid takeover assessment")
	}
	r.mu.Lock()
	r.reports[report.ID] = report
	r.mu.Unlock()
	r.audit.Append(audit.Entry{Actor: actor, Action: "takeover.assessment.submit", Target: report.ID, RequestID: requestID, At: now.UTC()})
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
