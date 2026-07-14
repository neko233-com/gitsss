// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestPreparePlanSnapshotsSQLiteWAL(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "gitea.db")
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec("CREATE TABLE issue (id INTEGER PRIMARY KEY, title TEXT); INSERT INTO issue(title) VALUES ('from WAL')"); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "app.ini")
	writeTestFile(t, configPath, "[database]\nDB_TYPE = sqlite3\nPATH = "+filepath.ToSlash(databasePath)+"\n")
	report, err := takeover.Assess(takeover.Options{AgentID: "sqlite-agent", ConfigPath: configPath, WorkPath: root, MaxEntries: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PreparePlan(report, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath := preparedPlanSourcePath(prepared.Plan, sqliteSnapshotKind)
	if snapshotPath == "" {
		prepared.Close()
		t.Fatal("SQLite snapshot source is missing")
	}
	snapshot, err := sql.Open("sqlite", sqliteReadOnlyDSN(snapshotPath))
	if err != nil {
		prepared.Close()
		t.Fatal(err)
	}
	var title string
	if err := snapshot.QueryRow("SELECT title FROM issue WHERE id = 1").Scan(&title); err != nil {
		snapshot.Close()
		prepared.Close()
		t.Fatal(err)
	}
	if err := snapshot.Close(); err != nil {
		prepared.Close()
		t.Fatal(err)
	}
	if title != "from WAL" {
		prepared.Close()
		t.Fatalf("snapshot title = %q", title)
	}
	temporaryDirectory := prepared.temporaryDirectory
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temporaryDirectory); !os.IsNotExist(err) {
		t.Fatalf("temporary snapshot directory remains: %v", err)
	}
}

func TestExportPreparedCleansSnapshotAfterFailure(t *testing.T) {
	temporaryDirectory := t.TempDir()
	prepared := &PreparedPlan{temporaryDirectory: temporaryDirectory}
	err := ExportPrepared(filepath.Join(t.TempDir(), "backup.gitsss"), "", prepared)
	if err == nil {
		t.Fatal("empty passphrase was accepted")
	}
	if _, err := os.Stat(temporaryDirectory); !os.IsNotExist(err) {
		t.Fatalf("temporary snapshot directory remains after failure: %v", err)
	}
}

func TestShadowAuditDetectsSQLiteChange(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "gitea.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("CREATE TABLE user (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO user(name) VALUES ('first')"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "app.ini")
	repositoriesPath := filepath.Join(root, "repositories")
	if err := os.Mkdir(repositoriesPath, 0o700); err != nil {
		database.Close()
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "[repository]\nROOT = "+filepath.ToSlash(repositoriesPath)+"\n[database]\nDB_TYPE = sqlite3\nPATH = "+filepath.ToSlash(databasePath)+"\n")
	options := takeover.Options{AgentID: "sqlite-shadow-agent", ConfigPath: configPath, WorkPath: root, MaxEntries: 1_000_000}
	report, err := takeover.Assess(options)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	prepared, err := PreparePlan(report, time.Now())
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	defer prepared.Close()
	target := filepath.Join(root, "isolated")
	verification := materializeShadowTestSnapshot(t, prepared.Plan, target)
	audit, err := auditShadowVerified(verification, target, options)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if !audit.ReadyForShadowMode {
		database.Close()
		t.Fatalf("unchanged SQLite snapshot was rejected: %#v", audit)
	}
	if _, err := database.Exec("UPDATE user SET name = 'second' WHERE id = 1"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	audit, err = auditShadowVerified(verification, target, options)
	if err != nil {
		t.Fatal(err)
	}
	if audit.ReadyForShadowMode || !hasShadowFinding(audit.Findings, "database.source_changed") {
		t.Fatalf("changed SQLite source was accepted: %#v", audit)
	}
}

func preparedPlanSourcePath(plan Plan, kind string) string {
	for _, source := range plan.sources {
		if source.entry.Kind == kind {
			return source.path
		}
	}
	return ""
}
