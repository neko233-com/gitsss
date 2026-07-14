// Copyright 2026 gitsss Authors. All rights reserved.

package migration

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestMaterializeBuildsStandaloneGenerationAndDetectsDrift(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "restored", "content")
	snapshotDirectory := filepath.Join(source, "data", "database-snapshot")
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(snapshotDirectory, "gitea.db")
	database, err := sql.Open("sqlite", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT, type INTEGER)`,
		`INSERT INTO user VALUES (1, 'artist', 0), (2, 'studio', 1)`,
		`CREATE TABLE repository (id INTEGER PRIMARY KEY, owner_id INTEGER, name TEXT, default_branch TEXT)`,
		`INSERT INTO repository VALUES (3, 2, 'game', 'main')`,
		`CREATE TABLE issue (id INTEGER PRIMARY KEY, repo_id INTEGER, name TEXT, is_pull INTEGER)`,
		`INSERT INTO issue VALUES (4, 3, 'lighting', 0), (5, 3, 'merge', 1)`,
		`CREATE TABLE pull_request (id INTEGER PRIMARY KEY, issue_id INTEGER)`,
		`INSERT INTO pull_request VALUES (6, 5)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(source, "data", "repositories", "studio", "game.git")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "HEAD"), []byte("ref: refs/heads/trunk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lfs := filepath.Join(source, "data", "lfs", "objects", "aa", "asset")
	if err := os.MkdirAll(filepath.Dir(lfs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lfs, []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	inventory, err := Inspect(source, now)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "materialized")
	active := filepath.Join(root, "active")
	result, err := Materialize(source, target, active, inventory.Digest, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Repositories != 1 || result.ImportedRows["user"] != 2 || result.ImportedRows["issue"] != 2 || result.Counts.PullRequests != 1 || result.ActivePath != active {
		t.Fatalf("materialization = %#v", result)
	}
	if _, err := Materialize(source, target, active, inventory.Digest, now); !errors.Is(err, ErrInvalidMaterialization) {
		t.Fatalf("existing target error = %v", err)
	}
	if _, err := VerifyMaterialization(target, inventory.Digest, result.ContentSHA256); err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(filepath.Join(target, "content", "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	users, err := store.List("source-import-user")
	if err != nil || len(users) != 2 {
		t.Fatalf("users = %#v, %v", users, err)
	}
	repositories, err := store.List("repository-catalog")
	if err != nil || len(repositories) != 1 {
		t.Fatalf("repositories = %#v, %v", repositories, err)
	}
	var catalog repositoryCatalogRecord
	if err := json.Unmarshal(repositories[0].Payload, &catalog); err != nil || catalog.DefaultBranch != "trunk" || catalog.Path != filepath.Join(active, "repositories", "studio-game.git") {
		t.Fatalf("catalog = %#v, %v", catalog, err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "content", "lfs", "objects", "aa", "asset")); err != nil || string(content) != "asset" {
		t.Fatalf("LFS content = %q, %v", content, err)
	}
	if err := os.WriteFile(filepath.Join(target, "content", "repositories", "studio-game.git", "HEAD"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyMaterialization(target, inventory.Digest, result.ContentSHA256); !errors.Is(err, ErrInvalidMaterialization) {
		t.Fatalf("drift error = %v", err)
	}
}

func TestMaterializeRejectsOverlappingTargets(t *testing.T) {
	root := t.TempDir()
	if _, err := Materialize(root, filepath.Join(root, "nested"), filepath.Join(root, "active"), strings.Repeat("0", 64), time.Now()); !errors.Is(err, ErrInvalidMaterialization) {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestMaterializeImportsPortableDatabaseSnapshot(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "restored", "content")
	directory := filepath.Join(source, "data", "database-snapshot")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(directory, "postgres.jsonl")
	content := "{\"format\":\"gitsss-database-v1\",\"database\":\"postgres\"}\n" +
		"{\"table\":\"public.user\",\"columns\":[\"id\",\"name\",\"type\"],\"types\":[\"bigint\",\"text\",\"integer\"]}\n" +
		"{\"table\":\"public.user\",\"values\":[{\"kind\":\"int\",\"data\":\"1\"},{\"kind\":\"string\",\"data\":\"YXJ0aXN0\"},{\"kind\":\"int\",\"data\":\"0\"}]}\n"
	if err := os.WriteFile(snapshot, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 2, 0, 0, 0, time.UTC)
	inventory, err := Inspect(source, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Materialize(source, filepath.Join(root, "materialized"), filepath.Join(root, "active"), inventory.Digest, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImportedRows["user"] != 1 || result.Counts.Users != 1 || result.SourceSHA256 == "" {
		t.Fatalf("portable result = %#v", result)
	}
}
