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
)

func TestAuditGitRepositoryDetectsObjectLoss(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	contentRoot := t.TempDir()
	relative := "data/repositories/team/game.git"
	repository := filepath.Join(contentRoot, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(repository), 0o700); err != nil {
		t.Fatal(err)
	}
	runAuditTestGit(t, gitExecutable, "", nil, "init", "--bare", repository)
	blob := runAuditTestGit(t, gitExecutable, repository, []byte("asset-data"), "hash-object", "-w", "--stdin")
	treeInput := []byte("100644 blob " + blob + "\tasset.txt\n")
	tree := runAuditTestGit(t, gitExecutable, repository, treeInput, "mktree")
	environment := []string{
		"GIT_AUTHOR_NAME=gitsss test", "GIT_AUTHOR_EMAIL=test@gitsss.local", "GIT_AUTHOR_DATE=2026-07-14T00:00:00Z",
		"GIT_COMMITTER_NAME=gitsss test", "GIT_COMMITTER_EMAIL=test@gitsss.local", "GIT_COMMITTER_DATE=2026-07-14T00:00:00Z",
	}
	command := exec.Command(gitExecutable, "-C", repository, "commit-tree", tree, "-m", "initial")
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("commit-tree: %v: %s", err, output)
	}
	commit := strings.TrimSpace(string(output))
	runAuditTestGit(t, gitExecutable, repository, nil, "update-ref", "refs/heads/main", commit)
	runAuditTestGit(t, gitExecutable, repository, nil, "symbolic-ref", "HEAD", "refs/heads/main")

	audit := auditGitRepository(gitExecutable, contentRoot, relative)
	if !audit.Valid || audit.Objects != 3 || audit.Refs != 2 || len(audit.ReferenceSHA256) != sha256.Size*2 {
		t.Fatalf("unexpected valid audit: %#v", audit)
	}
	alternatesPath := filepath.Join(repository, "objects", "info", "alternates")
	writeTestFile(t, alternatesPath, filepath.Join(contentRoot, "external-objects"))
	audit = auditGitRepository(gitExecutable, contentRoot, relative)
	if audit.Valid || !strings.Contains(audit.Error, "alternates") {
		t.Fatalf("object alternates were accepted: %#v", audit)
	}
	if err := os.Remove(alternatesPath); err != nil {
		t.Fatal(err)
	}
	objectPath := filepath.Join(repository, "objects", commit[:2], commit[2:])
	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	audit = auditGitRepository(gitExecutable, contentRoot, relative)
	if audit.Valid || audit.Error == "" {
		t.Fatalf("missing object was accepted: %#v", audit)
	}
}

func TestAuditLFSComparesOIDToContentDigest(t *testing.T) {
	content := []byte("lfs-object")
	digest := sha256.Sum256(content)
	oid := fmt.Sprintf("%x", digest)
	manifest := Manifest{Entries: []Entry{
		{Path: "data/lfs/aa/bb/" + oid, Kind: "lfs", Size: int64(len(content)), SHA256: oid},
		{Path: "data/lfs/cc/dd/" + strings.Repeat("f", 64), Kind: "lfs", Size: int64(len(content)), SHA256: oid},
		{Path: "data/lfs/metadata.json", Kind: "lfs", Size: 2, SHA256: strings.Repeat("a", 64)},
	}}
	audit := auditLFS(manifest)
	if audit.Objects != 2 || len(audit.Invalid) != 1 || audit.Invalid[0].Expected != strings.Repeat("f", 64) {
		t.Fatalf("unexpected LFS audit: %#v", audit)
	}
}

func runAuditTestGit(t *testing.T, executable, repository string, input []byte, arguments ...string) string {
	t.Helper()
	commandArguments := arguments
	if repository != "" {
		commandArguments = append([]string{"-C", repository}, arguments...)
	}
	command := exec.Command(executable, commandArguments...)
	command.Stdin = strings.NewReader(string(input))
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
