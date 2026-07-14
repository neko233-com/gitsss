// Copyright 2026 gitsss Authors. All rights reserved.

package state

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestStorePersistsDocumentsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "gitsss.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put("agent-jobs", "job-1", json.RawMessage(`{"status":"queued"}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	documents, err := reopened.List("agent-jobs")
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].ID != "job-1" || string(documents[0].Payload) != `{"status":"queued"}` {
		t.Fatalf("documents = %#v", documents)
	}
}

func TestStoreRejectsUnsafeDocumentsAndNonFiles(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Put("bad\nnamespace", "id", json.RawMessage(`{}`)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unsafe namespace error = %v", err)
	}
	if err := store.Put("safe", "id", json.RawMessage(`not-json`)); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("invalid JSON error = %v", err)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("directory database error = %v", err)
	}
}
