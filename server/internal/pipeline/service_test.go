// Copyright 2026 gitsss Authors. All rights reserved.

package pipeline

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestPersistentPipelineLifecycleRedactsLogsAndSurvivesRestart(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	service, err := NewPersistentService(log, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	run, err := service.Create("maintainer", "request-1", "game", strings.Repeat("a", 40), "go-verify", now)
	if err != nil || run.Status != StatusQueued {
		t.Fatalf("created run = %#v, %v", run, err)
	}
	claim, found, err := service.Claim("runner-1", "request-2", "runner-1", now.Add(time.Second))
	if err != nil || !found || claim.Run.Status != StatusRunning || len(claim.Template.Steps) != 2 {
		t.Fatalf("claim = %#v, %v, %v", claim, found, err)
	}
	secret := "gitsss_abcdefghijklmnopqrstuvwxyz012345"
	if _, err := service.Heartbeat(run.ID, "other-runner", "", now.Add(2*time.Second)); err != ErrConflict {
		t.Fatalf("wrong runner heartbeat error = %v", err)
	}
	if _, err := service.Heartbeat(run.ID, "runner-1", "starting "+secret[:15], now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Heartbeat(run.ID, "runner-1", secret[15:]+" done\n", now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	completed, err := service.Complete("runner-1", "request-3", run.ID, "runner-1", true, "", now.Add(4*time.Second))
	if err != nil || completed.Status != StatusSucceeded || strings.Contains(completed.Logs, secret) || !strings.Contains(completed.Logs, "[REDACTED_TOKEN]") {
		t.Fatalf("completed run = %#v, %v", completed, err)
	}
	reopened, err := NewPersistentService(log, store)
	if err != nil {
		t.Fatal(err)
	}
	runs := reopened.Runs()
	if len(runs) != 1 || runs[0].Status != StatusSucceeded || strings.Contains(runs[0].Logs, secret) {
		t.Fatalf("reloaded runs = %#v", runs)
	}
	entries := log.Entries()
	if len(entries) != 3 || entries[0].Action != "pipeline.complete" {
		t.Fatalf("audit entries = %#v", entries)
	}
}

func TestPipelineRetriesExpiredLeaseThenFailsClosed(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service, err := NewPersistentService(&audit.Log{}, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	run, err := service.Create("maintainer", "create", "game", strings.Repeat("b", 40), "go-verify", now)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= maximumAttempts; attempt++ {
		claim, found, err := service.Claim("runner", "claim", "runner", now.Add(time.Duration(attempt-1)*3*time.Minute))
		if err != nil || !found || claim.Run.ID != run.ID || claim.Run.Attempts != attempt {
			t.Fatalf("attempt %d claim = %#v, %v, %v", attempt, claim, found, err)
		}
	}
	if claim, found, err := service.Claim("runner", "expire", "runner", now.Add(9*time.Minute)); err != nil || found {
		t.Fatalf("expired claim = %#v, %v, %v", claim, found, err)
	}
	runs := service.Runs()
	if len(runs) != 1 || runs[0].Status != StatusFailed || !strings.Contains(runs[0].Error, "three attempts") {
		t.Fatalf("expired runs = %#v", runs)
	}
}

func TestTemplatesAreImmutableAndRequestsAreBounded(t *testing.T) {
	listed := Templates()
	if len(listed) != 3 || listed[0].ID != "gitsss-verify" {
		t.Fatalf("templates = %#v", listed)
	}
	listed[0].Steps[0].Command[0] = "powershell"
	unchanged := Templates()
	if unchanged[0].Steps[0].Command[0] != "go" {
		t.Fatalf("template commands were mutable: %#v", unchanged[0])
	}
}
