// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

func TestRegistryStoresLatestAssessmentAndAuditsSubmission(t *testing.T) {
	log := &audit.Log{}
	registry := NewRegistry(log)
	report := Report{ID: "assessment-0011223344556677", AgentID: "agent", ConfigDigest: "sha256:" + strings.Repeat("a", 64), AssessedAt: time.Now()}
	if _, err := registry.Submit("agent-token", "request", report, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(registry.Reports()) != 1 || len(log.Entries()) != 1 {
		t.Fatalf("reports/audit = %d/%d", len(registry.Reports()), len(log.Entries()))
	}
}
