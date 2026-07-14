// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
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

func TestPersistentRegistrySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitsss.db")
	store, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewPersistentRegistry(nil, store)
	if err != nil {
		t.Fatal(err)
	}
	report := Report{
		ID: "assessment-0011223344556677", AgentID: "agent-a",
		ConfigDigest: "sha256:" + strings.Repeat("b", 64), AssessedAt: time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC),
	}
	if _, err := registry.Submit("agent-a", "request", report, report.AssessedAt); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restarted, err := NewPersistentRegistry(nil, store)
	if err != nil {
		t.Fatal(err)
	}
	reports := restarted.Reports()
	if len(reports) != 1 || reports[0].ID != report.ID || reports[0].AgentID != "agent-a" {
		t.Fatalf("restarted reports = %#v", reports)
	}
}
