// Copyright 2026 gitsss Authors. All rights reserved.

package migration

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestInspectSQLiteBusinessInventory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "content")
	snapshotDirectory := filepath.Join(root, "data", "database-snapshot")
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(snapshotDirectory, "source.db")
	database, err := sql.Open("sqlite", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE user (id INTEGER PRIMARY KEY, type INTEGER); INSERT INTO user(type) VALUES (0), (0), (1)`,
		`CREATE TABLE repository (id INTEGER PRIMARY KEY); INSERT INTO repository DEFAULT VALUES`,
		`CREATE TABLE issue (id INTEGER PRIMARY KEY, is_pull INTEGER); INSERT INTO issue(is_pull) VALUES (0), (0), (1)`,
		`CREATE TABLE pull_request (id INTEGER PRIMARY KEY); INSERT INTO pull_request DEFAULT VALUES`,
		`CREATE TABLE comment (id INTEGER PRIMARY KEY); INSERT INTO comment DEFAULT VALUES`,
		`CREATE TABLE release (id INTEGER PRIMARY KEY); INSERT INTO release DEFAULT VALUES`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	inventory, err := Inspect(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Database != "sqlite3" || len(inventory.Digest) != 64 || inventory.Counts.Users != 2 || inventory.Counts.Organizations != 1 || inventory.Counts.Repositories != 1 || inventory.Counts.Issues != 2 || inventory.Counts.PullRequests != 1 || inventory.Counts.Comments != 1 || inventory.Counts.Releases != 1 || inventory.InspectedAt != now {
		t.Fatalf("inventory = %#v", inventory)
	}
}

func TestInspectPortableBusinessInventory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "content")
	snapshotDirectory := filepath.Join(root, "data", "database-snapshot")
	if err := os.MkdirAll(snapshotDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	content := "{\"format\":\"gitsss-database-v1\",\"database\":\"postgres\"}\n" +
		"{\"table\":\"public.user\",\"columns\":[\"id\",\"type\"],\"types\":[\"bigint\",\"integer\"]}\n" +
		"{\"table\":\"public.user\",\"values\":[{\"kind\":\"int\",\"data\":\"1\"},{\"kind\":\"int\",\"data\":\"0\"}]}\n" +
		"{\"table\":\"public.user\",\"values\":[{\"kind\":\"int\",\"data\":\"2\"},{\"kind\":\"bytes\",\"data\":\"" + encode("1") + "\"}]}\n" +
		"{\"table\":\"public.issue\",\"columns\":[\"id\",\"is_pull\"],\"types\":[\"bigint\",\"boolean\"]}\n" +
		"{\"table\":\"public.issue\",\"values\":[{\"kind\":\"int\",\"data\":\"1\"},{\"kind\":\"bool\",\"data\":\"false\"}]}\n" +
		"{\"table\":\"public.issue\",\"values\":[{\"kind\":\"int\",\"data\":\"2\"},{\"kind\":\"bool\",\"data\":\"true\"}]}\n" +
		"{\"table\":\"public.repository\",\"columns\":[\"id\"],\"types\":[\"bigint\"]}\n" +
		"{\"table\":\"public.repository\",\"values\":[{\"kind\":\"int\",\"data\":\"9\"}]}\n"
	if err := os.WriteFile(filepath.Join(snapshotDirectory, "postgres.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory, err := Inspect(root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Database != "postgres" || inventory.Counts.Users != 1 || inventory.Counts.Organizations != 1 || inventory.Counts.Repositories != 1 || inventory.Counts.Issues != 1 || inventory.Counts.PullRequests != 1 {
		t.Fatalf("inventory = %s", fmt.Sprintf("%#v", inventory))
	}
}

func TestInspectRejectsMultipleDatabaseSnapshots(t *testing.T) {
	root := filepath.Join(t.TempDir(), "content", "data", "database-snapshot")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.db", "two.db"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Inspect(filepath.Dir(filepath.Dir(root)), time.Now()); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v", err)
	}
}
