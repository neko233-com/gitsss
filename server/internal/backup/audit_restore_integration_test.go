// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestAuditRestoreVerifiesGitLFSAndDetectsCorruption(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	repositoriesPath := filepath.Join(directory, "repositories")
	repositoryPath := filepath.Join(repositoriesPath, "game.git")
	emptyRepositoryPath := filepath.Join(repositoriesPath, "empty.git")
	if err := os.Mkdir(repositoriesPath, 0o700); err != nil {
		t.Fatal(err)
	}
	runAuditTestGit(t, gitExecutable, "", nil, "init", "--bare", repositoryPath)
	runAuditTestGit(t, gitExecutable, "", nil, "init", "--bare", emptyRepositoryPath)
	blob := runAuditTestGit(t, gitExecutable, repositoryPath, []byte("game-object"), "hash-object", "-w", "--stdin")
	tree := runAuditTestGit(t, gitExecutable, repositoryPath, []byte("100644 blob "+blob+"\tasset.bin\n"), "mktree")
	commit := runAuditTestGit(t, gitExecutable, repositoryPath, nil,
		"-c", "user.name=gitsss test", "-c", "user.email=test@gitsss.local", "commit-tree", tree, "-m", "initial")
	runAuditTestGit(t, gitExecutable, repositoryPath, nil, "update-ref", "refs/heads/main", commit)
	runAuditTestGit(t, gitExecutable, repositoryPath, nil, "symbolic-ref", "HEAD", "refs/heads/main")
	writeTestFile(t, configPath, "[database]\nDB_TYPE = sqlite3\n")
	lfsContent := []byte("deterministic-game-texture")
	lfsDigest := sha256.Sum256(lfsContent)
	lfsOID := fmt.Sprintf("%x", lfsDigest)
	lfsPath := filepath.Join(directory, "lfs")
	lfsObjectPath := filepath.Join(lfsPath, "objects", "aa", "bb", lfsOID)
	if err := os.MkdirAll(filepath.Dir(lfsObjectPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestBytes(t, lfsObjectPath, lfsContent)
	report := takeover.Report{
		ID: "assessment-audit-integration", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("b", 64),
		Assets: []takeover.Asset{
			{Kind: "repositories", Path: repositoriesPath, Exists: true, Readable: true},
			{Kind: "lfs", Path: lfsPath, Exists: true, Readable: true},
		},
	}
	started := time.Now()
	plan, err := BuildPlan(report, started)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(directory, "audit.gitsss-backup")
	const passphrase = "audit integration passphrase"
	if err := Export(backupPath, passphrase, plan); err != nil {
		t.Fatal(err)
	}
	verification, err := Verify(backupPath, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(directory, "isolated")
	if _, err := RestoreIsolatedExact(backupPath, passphrase, targetPath, report.ID, verification.ContainerSHA256); err != nil {
		t.Fatal(err)
	}
	audit, err := AuditRestoreExact(backupPath, passphrase, targetPath, report.ID, verification.ContainerSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !audit.ReadyForShadowMode || audit.Files.Identical != len(plan.Manifest.Entries) || len(audit.Git) != 2 || !audit.Git[0].Valid || !audit.Git[1].Valid || audit.LFS.Objects != 1 || len(audit.LFS.Invalid) != 0 {
		t.Fatalf("unexpected ready audit: %#v", audit)
	}
	restoredLFS := filepath.Join(targetPath, "content", "data", "lfs", "objects", "aa", "bb", lfsOID)
	writeTestFile(t, restoredLFS, "corrupt")
	audit, err = AuditRestoreExact(backupPath, passphrase, targetPath, report.ID, verification.ContainerSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if audit.ReadyForShadowMode || audit.Files.Conflicts != 1 {
		t.Fatalf("corruption was accepted: %#v", audit)
	}
	t.Logf("verified Git, LFS and corruption detection in %s", time.Since(started))
}
