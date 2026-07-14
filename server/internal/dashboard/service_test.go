// Copyright 2026 gitsss Authors. All rights reserved.

package dashboard

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestServiceKeepsRecentEventsAndAuditsPreviewClear(t *testing.T) {
	log := &audit.Log{}
	service := NewService(2, []Event{{ID: "newest"}, {ID: "older"}, {ID: "oldest"}}, log)

	snapshot := service.Snapshot()
	if len(snapshot.Events) != 2 || snapshot.Events[0].ID != "newest" || snapshot.Events[1].ID != "older" {
		t.Fatalf("events = %#v, want newest-first capacity of 2", snapshot.Events)
	}

	if err := service.ClearPreview("agent-1", "request-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if events := service.Snapshot().Events; len(events) != 0 {
		t.Fatalf("events = %#v, want empty preview", events)
	}
	if entries := log.Entries(); len(entries) != 1 || entries[0].Action != "events.preview.clear" {
		t.Fatalf("audit entries = %#v, want clear event", entries)
	}
}

func TestServiceDoesNotClearPreviewWhenAuditCannotPersist(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	vault, err := state.OpenVault(filepath.Join(root, "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	log, err := audit.NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(2, []Event{{ID: "event-1"}}, log)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearPreview("agent-1", "request-1", time.Now()); !errors.Is(err, audit.ErrState) {
		t.Fatalf("clear error = %v", err)
	}
	if events := service.Snapshot().Events; len(events) != 1 || events[0].ID != "event-1" {
		t.Fatalf("events after failed audit = %#v", events)
	}
}
