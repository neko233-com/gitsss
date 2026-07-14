// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAssessBuildsBoundedRedactedInventory(t *testing.T) {
	root := t.TempDir()
	configDirectory := filepath.Join(root, "custom", "conf")
	if err := os.MkdirAll(filepath.Join(root, "data", "gitea-repositories", "team", "game.git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "gitea-repositories", "team", "game.git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDirectory, "app.ini")
	configuration := `[server]
ROOT_URL = http://git.internal.example/path?token=hidden
APP_DATA_PATH = data
[database]
DB_TYPE = postgres
HOST = 10.0.0.8:5432
NAME = gitea
PASSWD = super-secret
[repository]
ROOT = data/gitea-repositories
[actions]
ENABLED = false
`
	if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	report, err := Assess(Options{AgentID: "agent-1", ConfigPath: configPath, MaxEntries: 100, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if report.Database.Location != "remote-service" || report.Database.Type != "postgres" {
		t.Fatalf("database = %#v", report.Database)
	}
	if report.PublicOrigin != "http://git.internal.example" || report.SecretFieldCount != 1 {
		t.Fatalf("report origin/secrets = %#v", report)
	}
	if strings.Contains(report.ConfigDigest, "super-secret") || report.ActionsEnabled {
		t.Fatalf("report leaked secret or actions state = %#v", report)
	}
	if !report.Assets[0].Exists || report.Assets[0].Files != 1 {
		t.Fatalf("repository asset = %#v", report.Assets[0])
	}
	if len(report.Findings) == 0 {
		t.Fatal("expected findings for HTTP and disabled actions")
	}
}

func TestAssessDoesNotFollowSymlinksAndCapsScan(t *testing.T) {
	root := t.TempDir()
	repositories := filepath.Join(root, "repositories")
	if err := os.MkdirAll(repositories, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		if err := os.WriteFile(filepath.Join(repositories, string(rune('a'+index))), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(root, "app.ini")
	if err := os.WriteFile(config, []byte("[repository]\nROOT = "+repositories+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Assess(Options{AgentID: "agent", ConfigPath: config, WorkPath: root, MaxEntries: 2})
	if err != nil {
		t.Fatal(err)
	}
	if report.Assets[0].Files != 2 || !report.Assets[0].Truncated {
		t.Fatalf("bounded asset = %#v", report.Assets[0])
	}
}

func TestParseINIHonorsQuotedCommentCharacters(t *testing.T) {
	configuration, err := parseINI(strings.NewReader("[database]\nPASSWD = `p;a#s` ; comment\nHOST = localhost:5432 ; comment\n"))
	if err != nil {
		t.Fatal(err)
	}
	if value(configuration, "database", "PASSWD") != "p;a#s" || value(configuration, "database", "HOST") != "localhost:5432" {
		t.Fatalf("configuration = %#v", configuration)
	}
}
