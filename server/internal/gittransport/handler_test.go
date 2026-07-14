// Copyright 2026 gitsss Authors. All rights reserved.

package gittransport

import (
	"io"
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
	"github.com/neko233-com/gitsss/server/internal/repository"
)

func TestHandlerClonesAndPushesSmartHTTP(t *testing.T) {
	remote := seedBareRepository(t)
	repositories, err := repository.NewService([]repository.Repository{{ID: "game", Name: "Game", DefaultBranch: "main", Path: remote}})
	if err != nil {
		t.Fatal(err)
	}
	log := &audit.Log{}
	verifier := auth.NewVerifier(
		auth.NewRecord("reader", "clone-secret-token-2026", time.Time{}, "repositories:clone"),
		auth.NewRecord("writer", "push-secret-token-2026", time.Time{}, "repositories:push"),
	)
	handler := NewHandler(verifier, repositories, log)
	server := httptest.NewServer(handler)
	defer server.Close()

	clone := filepath.Join(t.TempDir(), "clone")
	runGitTest(t, "", "-c", "http.extraHeader=Authorization: Bearer clone-secret-token-2026", "clone", "--filter=blob:none", server.URL+"/git/game.git", clone)
	if content, err := os.ReadFile(filepath.Join(clone, "README.md")); err != nil || strings.ReplaceAll(string(content), "\r\n", "\n") != "# Game\n" {
		t.Fatalf("cloned README = %q, error = %v", content, err)
	}
	if value := strings.TrimSpace(runGitTest(t, clone, "config", "--bool", "remote.origin.promisor")); value != "true" {
		t.Fatalf("partial clone promisor = %q", value)
	}
	if err := os.WriteFile(filepath.Join(clone, "README.md"), []byte("# Game\nship\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, clone, "config", "user.name", "gitsss Test")
	runGitTest(t, clone, "config", "user.email", "test@gitsss.invalid")
	runGitTest(t, clone, "add", "README.md")
	runGitTest(t, clone, "commit", "-m", "ship")
	runGitTest(t, clone, "-c", "http.extraHeader=Authorization: Bearer push-secret-token-2026", "push", "origin", "main")
	output := runGitTest(t, remote, "show", "main:README.md")
	if output != "# Game\nship\n" {
		t.Fatalf("pushed README = %q", output)
	}
	var received bool
	for _, entry := range log.Entries() {
		if entry.Actor == "writer" && entry.Action == "git.receive-pack" && entry.Result == "accepted" {
			received = true
		}
	}
	if !received {
		t.Fatalf("receive audit missing from %#v", log.Entries())
	}
}

func TestHandlerRequiresExactAuthenticationAndScope(t *testing.T) {
	remote := seedBareRepository(t)
	repositories, err := repository.NewService([]repository.Repository{{ID: "game", Name: "Game", Path: remote}})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(auth.NewVerifier(auth.NewRecord("reader", "clone-secret-token-2026", time.Time{}, "repositories:clone")), repositories, &audit.Log{})
	server := httptest.NewServer(handler)
	defer server.Close()

	for _, test := range []struct {
		authorization string
		path          string
		status        int
	}{
		{authorization: "Bearerclone-secret-token-2026", path: "/git/game.git/info/refs?service=git-upload-pack", status: http.StatusUnauthorized},
		{authorization: "Bearer clone-secret-token-2026", path: "/git/game.git/info/refs?service=git-receive-pack", status: http.StatusForbidden},
		{authorization: "Bearer clone-secret-token-2026", path: "/git/../game.git/info/refs?service=git-upload-pack", status: http.StatusNotFound},
	} {
		request, err := http.NewRequest(http.MethodGet, server.URL+test.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", test.authorization)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Fatalf("%s status = %d, want %d", test.path, response.StatusCode, test.status)
		}
	}
}

func seedBareRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGitTest(t, "", "init", "--bare", "--initial-branch=main", remote)
	runGitTest(t, remote, "config", "uploadpack.allowFilter", "true")
	runGitTest(t, remote, "config", "uploadpack.allowAnySHA1InWant", "true")
	source := filepath.Join(root, "source")
	runGitTest(t, "", "init", "--initial-branch=main", source)
	runGitTest(t, source, "config", "user.name", "gitsss Test")
	runGitTest(t, source, "config", "user.email", "test@gitsss.invalid")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# Game\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, source, "add", "README.md")
	runGitTest(t, source, "commit", "-m", "initial")
	runGitTest(t, source, "remote", "add", "origin", remote)
	runGitTest(t, source, "push", "origin", "main")
	return remote
}

func runGitTest(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	commandArguments := arguments
	if directory != "" {
		commandArguments = append([]string{"-C", directory}, arguments...)
	}
	command := exec.Command("git", commandArguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(commandArguments, " "), err, output)
	}
	return string(output)
}
