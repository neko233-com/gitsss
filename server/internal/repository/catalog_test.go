// Copyright 2026 gitsss Authors. All rights reserved.

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestPersistentServiceCreatesAndReloadsHostedRepository(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "state", "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	repositoryRoot := filepath.Join(root, "repositories")
	service, err := NewPersistentService(log, store, repositoryRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	created, err := service.Create("admin", "request-1", CreateInput{ID: "art-team", Name: "Art Team", DefaultBranch: "main"}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Hosted || created.CreatedAt != createdAt || filepath.Base(created.Path) != "art-team.git" {
		t.Fatalf("created repository = %#v", created)
	}
	if output, err := runGit(context.Background(), created.Path, 256, "symbolic-ref", "HEAD"); err != nil || string(output) != "refs/heads/main\n" {
		t.Fatalf("bare HEAD = %q, error = %v", output, err)
	}
	reopened, err := NewPersistentService(log, store, repositoryRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := reopened.Resolve("art-team")
	if err != nil || resolved.Path != created.Path || !resolved.Hosted {
		t.Fatalf("reloaded repository = %#v, error = %v", resolved, err)
	}
	entries := log.Entries()
	if len(entries) != 1 || entries[0].Action != "repository.create" || entries[0].Target != "repository:art-team" {
		t.Fatalf("audit entries = %#v", entries)
	}
}

func TestPersistentServiceRejectsUnsafeAndDuplicateRepositoryIDs(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	repositoryRoot := filepath.Join(root, "repositories")
	service, err := NewPersistentService(&audit.Log{}, store, repositoryRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create("admin", "request-1", CreateInput{ID: "../escape", Name: "Escape"}, time.Now()); err != ErrInvalidPath {
		t.Fatalf("unsafe create error = %v", err)
	}
	if _, err := service.Create("admin", "request-2", CreateInput{ID: "safe", Name: "Safe"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create("admin", "request-3", CreateInput{ID: "safe", Name: "Duplicate"}, time.Now()); err != ErrExists {
		t.Fatalf("duplicate create error = %v", err)
	}
	entries, err := os.ReadDir(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "safe.git" {
		t.Fatalf("repository root entries = %#v", entries)
	}
}
