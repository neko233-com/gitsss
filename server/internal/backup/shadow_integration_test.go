// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestAuditShadowComparesLiveConfigAssetsAndGitRefs(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	dataPath := filepath.Join(root, "data")
	repositoriesPath := filepath.Join(root, "repositories")
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(repositoriesPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dataPath, "gitea.db"), "sqlite snapshot")
	repositoryPath := filepath.Join(repositoriesPath, "game.git")
	runAuditTestGit(t, gitExecutable, "", nil, "init", "--bare", repositoryPath)
	firstCommit := createShadowTestCommit(t, gitExecutable, repositoryPath, "first")
	runAuditTestGit(t, gitExecutable, repositoryPath, nil, "update-ref", "refs/heads/main", firstCommit)
	configPath := filepath.Join(root, "app.ini")
	writeTestFile(t, configPath, "[server]\nAPP_DATA_PATH = "+filepath.ToSlash(dataPath)+"\n[repository]\nROOT = "+filepath.ToSlash(repositoriesPath)+"\n[database]\nDB_TYPE = sqlite3\nPATH = "+filepath.ToSlash(filepath.Join(dataPath, "gitea.db"))+"\n")
	options := takeover.Options{AgentID: "shadow-agent", ConfigPath: configPath, WorkPath: root, MaxEntries: 1_000_000}
	report, err := takeover.Assess(options)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(report, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "isolated")
	verification := materializeShadowTestSnapshot(t, plan, target)
	audit, err := auditShadowVerified(verification, target, options)
	if err != nil {
		t.Fatal(err)
	}
	if !audit.ReadyForShadowMode || !audit.ConfigMatches || !audit.RestoreReady || audit.RepositorySummary.Matched != 1 || audit.RepositorySummary.Mismatched != 0 {
		t.Fatalf("unexpected ready shadow audit: %#v", audit)
	}

	secondCommit := createShadowTestCommit(t, gitExecutable, repositoryPath, "second")
	runAuditTestGit(t, gitExecutable, repositoryPath, nil, "update-ref", "refs/heads/main", secondCommit)
	audit, err = auditShadowVerified(verification, target, options)
	if err != nil {
		t.Fatal(err)
	}
	if audit.ReadyForShadowMode || audit.RepositorySummary.Mismatched != 1 || len(audit.Repositories) != 1 || !hasShadowFinding(audit.Findings, "repositories.refs_changed") {
		t.Fatalf("source Git drift was accepted: %#v", audit)
	}
}

func TestAuditShadowBlocksRemoteDatabaseWithoutSnapshot(t *testing.T) {
	root := t.TempDir()
	repositoriesPath := filepath.Join(root, "repositories")
	if err := os.Mkdir(repositoriesPath, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "app.ini")
	writeTestFile(t, configPath, "[repository]\nROOT = "+filepath.ToSlash(repositoriesPath)+"\n[database]\nDB_TYPE = postgres\nHOST = database.internal:5432\nNAME = gitea\n")
	options := takeover.Options{AgentID: "remote-db-agent", ConfigPath: configPath, WorkPath: root, MaxEntries: 1_000_000}
	report, err := takeover.Assess(options)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(report, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "isolated")
	verification := materializeShadowTestSnapshot(t, plan, target)
	audit, err := auditShadowVerified(verification, target, options)
	if err != nil {
		t.Fatal(err)
	}
	if audit.ReadyForShadowMode || !hasShadowFinding(audit.Findings, "database.snapshot_unavailable") {
		t.Fatalf("remote database without snapshot was accepted: %#v", audit)
	}
}

func TestCompactShadowTextBoundsValidUTF8(t *testing.T) {
	value := strings.Repeat("仓", maxShadowDetailBytes)
	compact := compactShadowText(value)
	if len(compact) > maxShadowDetailBytes || !utf8.ValidString(compact) {
		t.Fatalf("invalid compact shadow text: bytes=%d valid=%t", len(compact), utf8.ValidString(compact))
	}
	if compactShadowText("short") != "short" {
		t.Fatal("short shadow text changed")
	}
	if compact := compactShadowText("bad\xffvalue"); !utf8.ValidString(compact) {
		t.Fatal("invalid source text was not repaired")
	}
}

func createShadowTestCommit(t *testing.T, gitExecutable, repositoryPath, content string) string {
	t.Helper()
	blob := runAuditTestGit(t, gitExecutable, repositoryPath, []byte(content), "hash-object", "-w", "--stdin")
	tree := runAuditTestGit(t, gitExecutable, repositoryPath, []byte("100644 blob "+blob+"\tasset.bin\n"), "mktree")
	return runAuditTestGit(t, gitExecutable, repositoryPath, nil,
		"-c", "user.name=gitsss test", "-c", "user.email=test@gitsss.local", "commit-tree", tree, "-m", content)
}

func hasShadowFinding(findings []ShadowFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func materializeShadowTestSnapshot(t *testing.T, plan Plan, target string) Verification {
	t.Helper()
	contentRoot := filepath.Join(target, "content")
	for _, source := range plan.sources {
		content, err := os.ReadFile(source.path)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(contentRoot, filepath.FromSlash(source.entry.Path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Verification{Manifest: plan.Manifest, Files: len(plan.Manifest.Entries)}
}
