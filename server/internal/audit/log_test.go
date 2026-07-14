// Copyright 2026 gitsss Authors. All rights reserved.

package audit

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestPersistentLogEncryptsAndSurvivesRestart(t *testing.T) {
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
	log, err := NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{
		Actor: "admin@example.com", Action: "takeover.activate", Target: "private/repository",
		RequestID: "request-1", Result: "success", At: time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
	}
	if err := log.Append(entry); err != nil {
		t.Fatal(err)
	}
	documents, err := store.List(stateNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || strings.Contains(string(documents[0].Payload), entry.Actor) || strings.Contains(string(documents[0].Payload), entry.Target) {
		t.Fatalf("encrypted documents exposed audit content: %#v", documents)
	}
	reopened, err := NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	entries := reopened.Entries()
	if len(entries) != 1 || entries[0] != entry {
		t.Fatalf("reopened entries = %#v", entries)
	}
}

func TestPersistentLogCleanupSurvivesRestart(t *testing.T) {
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
	log, err := NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []Entry{
		{Actor: "admin", Action: "old.action", Target: "old", At: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		{Actor: "admin", Action: "new.action", Target: "new", At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	} {
		if err := log.Append(entry); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := log.Cleanup(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	reopened, err := NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	entries := reopened.Entries()
	if len(entries) != 1 || entries[0].Action != "new.action" {
		t.Fatalf("entries after cleanup = %#v", entries)
	}
}

func TestPersistentLogRejectsWrongKeyAndTampering(t *testing.T) {
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
	log, err := NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(Entry{Actor: "admin", Action: "secure.action", Target: "repository", At: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	wrongVault, err := state.OpenVault(filepath.Join(root, "wrong.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewPersistentLog(store, wrongVault); !errors.Is(err, ErrState) {
		t.Fatalf("wrong key error = %v", err)
	}
	documents, err := store.List(stateNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(stateNamespace, documents[0].ID, json.RawMessage(`{"v":1,"nonce":"AA","ciphertext":"AA"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPersistentLog(store, vault); !errors.Is(err, ErrState) {
		t.Fatalf("tamper error = %v", err)
	}
}
