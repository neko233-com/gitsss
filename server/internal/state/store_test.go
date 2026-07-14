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

func TestStoreDeletesDocumentsInOneNamespace(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, document := range []struct {
		namespace string
		id        string
	}{
		{namespace: "audit-entries", id: "audit-1"},
		{namespace: "audit-entries", id: "audit-2"},
		{namespace: "agent-jobs", id: "audit-1"},
	} {
		if err := store.Put(document.namespace, document.id, json.RawMessage(`{"status":"stored"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteMany("audit-entries", []string{"audit-1", "audit-2"}); err != nil {
		t.Fatal(err)
	}
	auditDocuments, err := store.List("audit-entries")
	if err != nil {
		t.Fatal(err)
	}
	jobDocuments, err := store.List("agent-jobs")
	if err != nil {
		t.Fatal(err)
	}
	if len(auditDocuments) != 0 || len(jobDocuments) != 1 {
		t.Fatalf("audit/job documents = %d/%d", len(auditDocuments), len(jobDocuments))
	}
	if err := store.DeleteMany("bad\nnamespace", []string{"audit-1"}); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unsafe delete error = %v", err)
	}
}
