// Copyright 2026 gitsss Authors. All rights reserved.

package dashboard

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
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
	if events := service.SnapshotFor("agent-1").Events; len(events) != 0 {
		t.Fatalf("events = %#v, want empty preview", events)
	}
	if entries := log.Entries(); len(entries) != 1 || entries[0].Action != "events.preview.clear" {
		t.Fatalf("audit entries = %#v, want clear event", entries)
	}
}

func TestPersistentServiceIsolatesEncryptedLRUPreviewsAcrossRestart(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vault, err := state.OpenVault(filepath.Join(root, "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPersistentService(2, []Event{{ID: "newest", Action: "private change"}, {ID: "older"}}, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.TouchFor("user-a", Event{ID: "build", Action: "build failed", Actor: "CI"}); err != nil {
		t.Fatal(err)
	}
	if err := service.TouchFor("user-a", Event{ID: "newest", Action: "updated private change"}); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearPreview("user-b", "request-clear", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	documents, err := store.List(previewStateNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("preview documents = %d", len(documents))
	}
	for _, document := range documents {
		payload := string(document.Payload)
		if strings.Contains(payload, "user-a") || strings.Contains(payload, "private change") || strings.Contains(payload, "build failed") {
			t.Fatalf("preview document exposed content: %s", payload)
		}
	}
	reopened, err := NewPersistentService(2, []Event{{ID: "newest", Action: "private change"}, {ID: "older"}}, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	userA := reopened.SnapshotFor("user-a").Events
	if len(userA) != 2 || userA[0].ID != "newest" || userA[1].ID != "build" || userA[0].Action != "updated private change" {
		t.Fatalf("user-a preview = %#v", userA)
	}
	if events := reopened.SnapshotFor("user-b").Events; len(events) != 0 {
		t.Fatalf("user-b preview = %#v", events)
	}
	if events := reopened.SnapshotFor("user-c").Events; len(events) != 2 || events[0].ID != "newest" {
		t.Fatalf("new viewer preview = %#v", events)
	}
}

func TestPersistentServiceRejectsTamperedPreview(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vault, err := state.OpenVault(filepath.Join(root, "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPersistentService(2, nil, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.TouchFor("user-a", Event{ID: "event-1"}); err != nil {
		t.Fatal(err)
	}
	documents, err := store.List(previewStateNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(previewStateNamespace, documents[0].ID, json.RawMessage(`{"v":1,"nonce":"AA","ciphertext":"AA"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPersistentService(2, nil, &audit.Log{}, store, vault); !errors.Is(err, ErrState) {
		t.Fatalf("tampered preview error = %v", err)
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
	if events := service.SnapshotFor("agent-1").Events; len(events) != 1 || events[0].ID != "event-1" {
		t.Fatalf("events after failed audit = %#v", events)
	}
}
