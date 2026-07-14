// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/api"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/pipeline"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestPipelineWorkerClonesExecutesAndCompletesBuiltInTemplate(t *testing.T) {
	remote, commit := createPipelineRepository(t)
	repositories, err := repository.NewService([]repository.Repository{{ID: "game", Name: "Game", Path: remote, DefaultBranch: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	pipelines, err := pipeline.NewPersistentService(log, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pipelines.Create("maintainer", "create", "game", commit, "go-verify", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	token := "runner-secret-token-2026"
	verifier := auth.NewVerifier(auth.NewRecord("runner-1", token, time.Time{}, "repositories:clone", "pipelines:work"))
	server := httptest.NewServer(api.NewServer(verifier, api.Dependencies{Audit: log, Pipelines: pipelines, Repositories: repositories}).Handler())
	defer server.Close()
	workspace := filepath.Join(root, "runner-workspace")
	worker := newPipelineWorker(server.URL, token, "runner-1", workspace, server.Client(), nil)
	outcome, err := worker.Cycle(context.Background())
	if err != nil || !outcome.Found || outcome.ExecuteErr != nil || outcome.Completed.Status != pipeline.StatusSucceeded {
		t.Fatalf("pipeline outcome = %#v, %v", outcome, err)
	}
	runs := pipelines.Runs()
	if len(runs) != 1 || runs[0].Status != pipeline.StatusSucceeded || !strings.Contains(runs[0].Logs, "Go tests") {
		t.Fatalf("pipeline runs = %#v", runs)
	}
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		t.Fatalf("workspace entries = %#v, %v", entries, err)
	}
}

func TestSafeRunnerEnvironmentDoesNotExposeAgentTokens(t *testing.T) {
	t.Setenv("GITSSS_RUNNER_TOKEN", "runner-secret")
	t.Setenv("GITSSS_AGENT_WORK_TOKEN", "agent-secret")
	for _, item := range safeRunnerEnvironment() {
		if strings.HasPrefix(item, "GITSSS_RUNNER_TOKEN=") || strings.HasPrefix(item, "GITSSS_AGENT_WORK_TOKEN=") || strings.Contains(item, "runner-secret") || strings.Contains(item, "agent-secret") {
			t.Fatalf("safe environment leaked a Token: %s", item)
		}
	}
}

func createPipelineRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	runPipelineGit(t, "", "init", "--initial-branch=main", source)
	runPipelineGit(t, source, "config", "user.name", "gitsss Test")
	runPipelineGit(t, source, "config", "user.email", "test@gitsss.invalid")
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.com/game\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "game_test.go"), []byte("package game\n\nimport \"testing\"\n\nfunc TestGame(t *testing.T) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runPipelineGit(t, source, "add", ".")
	runPipelineGit(t, source, "commit", "-m", "initial")
	commit := strings.TrimSpace(runPipelineGit(t, source, "rev-parse", "HEAD"))
	remote := filepath.Join(root, "remote.git")
	runPipelineGit(t, "", "clone", "--bare", source, remote)
	return remote, commit
}

func runPipelineGit(t *testing.T, directory string, arguments ...string) string {
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
