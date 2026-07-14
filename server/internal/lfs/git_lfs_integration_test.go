// Copyright 2026 gitsss Authors. All rights reserved.

package lfs

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/gittransport"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

func TestGitLFSClientPushesAndClonesGameAsset(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "game.git")
	runGitLFS(t, "", "init", "--bare", "--initial-branch=main", remote)
	repositories, err := repository.NewService([]repository.Repository{{ID: "game", Name: "Game", Path: remote, DefaultBranch: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	log := &audit.Log{}
	verifier := auth.NewVerifier(
		auth.NewRecord("writer", "push-token", time.Time{}, "repositories:push", "repositories:clone"),
		auth.NewRecord("reader", "clone-token", time.Time{}, "repositories:clone"),
	)
	lfsHandler, err := NewHandler(Options{Root: filepath.Join(root, "lfs")}, verifier, repositories, storage.NewService(1<<30, nil), log)
	if err != nil {
		t.Fatal(err)
	}
	gitHandler := gittransport.NewHandler(verifier, repositories, log)
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".git/info/lfs/") {
			lfsHandler.ServeHTTP(w, r)
			return
		}
		gitHandler.ServeHTTP(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	source := filepath.Join(root, "source")
	runGitLFS(t, "", "init", "--initial-branch=main", source)
	runGitLFS(t, source, "config", "user.name", "gitsss Test")
	runGitLFS(t, source, "config", "user.email", "test@gitsss.invalid")
	runGitLFS(t, source, "lfs", "install", "--local")
	runGitLFS(t, source, "lfs", "track", "*.psd")
	asset := bytes.Repeat([]byte{'P', 'S', 'D', 0, 1, 2, 3, 4}, 32<<10)
	if err := os.WriteFile(filepath.Join(source, "hero.psd"), asset, 0o600); err != nil {
		t.Fatal(err)
	}
	runGitLFS(t, source, "add", ".gitattributes", "hero.psd")
	runGitLFS(t, source, "commit", "-m", "add hero asset")
	runGitLFS(t, source, "remote", "add", "origin", server.URL+"/git/game.git")
	runGitLFS(t, source, "-c", "http.extraHeader=Authorization: Bearer push-token", "push", "origin", "main")

	clone := filepath.Join(root, "clone")
	runGitLFS(t, "",
		"-c", "http.extraHeader=Authorization: Bearer clone-token",
		"-c", "filter.lfs.process=git-lfs filter-process",
		"-c", "filter.lfs.smudge=git-lfs smudge -- %f",
		"-c", "filter.lfs.required=true",
		"clone", server.URL+"/git/game.git", clone,
	)
	cloned, err := os.ReadFile(filepath.Join(clone, "hero.psd"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cloned, asset) {
		t.Fatalf("cloned PSD size = %d, want %d exact bytes", len(cloned), len(asset))
	}
	oid := objectOID(asset)
	if info, err := regularFileInfo(lfsHandler.objectPath(oid)); err != nil || info.Size() != int64(len(asset)) {
		t.Fatalf("stored LFS object = %#v, %v", info, err)
	}
}

func runGitLFS(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	if directory != "" {
		arguments = append([]string{"-C", directory}, arguments...)
	}
	output, err := exec.Command("git", arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}
