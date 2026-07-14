// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
)

func TestAgentWorkerRetriesCompletionWithoutReexecution(t *testing.T) {
	var claims atomic.Int32
	var completions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer work-token" {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/api/v1/agent/jobs/claim":
			claims.Add(1)
			writeWorkerJSON(t, response, agentjob.Job{
				ID: "job-1", Kind: agentjob.KindAssessment, Status: agentjob.StatusRunning,
				Input: json.RawMessage(`{}`), ClaimedBy: "agent-1",
			})
		case "/api/v1/agent/jobs/job-1/complete":
			if completions.Add(1) == 1 {
				http.Error(response, "temporary failure", http.StatusInternalServerError)
				return
			}
			writeWorkerJSON(t, response, agentjob.Job{ID: "job-1", Status: agentjob.StatusSucceeded})
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	var executions atomic.Int32
	worker := newAgentWorker(server.URL, "work-token", "agent-1", server.Client(), func(job agentjob.Job, agentID string) (json.RawMessage, error) {
		executions.Add(1)
		return json.RawMessage(`{"ok":true}`), nil
	})
	if outcome, err := worker.Cycle(context.Background()); err == nil || !outcome.Found || worker.pending == nil {
		t.Fatalf("first cycle = %#v, %v, pending %#v", outcome, err, worker.pending)
	}
	outcome, err := worker.Cycle(context.Background())
	if err != nil || !outcome.Found || outcome.Completed.Status != agentjob.StatusSucceeded || worker.pending != nil {
		t.Fatalf("retry cycle = %#v, %v, pending %#v", outcome, err, worker.pending)
	}
	if claims.Load() != 1 || completions.Load() != 2 || executions.Load() != 1 {
		t.Fatalf("claims=%d completions=%d executions=%d", claims.Load(), completions.Load(), executions.Load())
	}
}

func TestAgentWorkerCancelsInFlightClaim(t *testing.T) {
	arrived := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		close(arrived)
		select {
		case <-request.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer server.Close()
	worker := newAgentWorker(server.URL, "work-token", "agent-1", server.Client(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := worker.Cycle(ctx)
		result <- err
	}()
	<-arrived
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("claim cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight claim did not stop after cancellation")
	}
}

func TestAgentLoopUsesBoundedBackoff(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		maximum    time.Duration
		expected   []time.Duration
		errorDelay bool
	}{
		{name: "idle", status: http.StatusNoContent, maximum: 4 * time.Second, expected: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}},
		{name: "error", status: http.StatusServiceUnavailable, maximum: 3 * time.Second, expected: []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second}, errorDelay: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(test.status)
			}))
			defer server.Close()
			worker := newAgentWorker(server.URL, "work-token", "agent-1", server.Client(), nil)
			delays := make([]time.Duration, 0, len(test.expected))
			wait := func(_ context.Context, delay time.Duration) bool {
				delays = append(delays, delay)
				return len(delays) < len(test.expected)
			}
			idleMax, errorMax := test.maximum, 10*time.Second
			if test.errorDelay {
				idleMax, errorMax = 10*time.Second, test.maximum
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := runAgentLoop(context.Background(), worker, time.Second, idleMax, errorMax, wait, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			if len(delays) != len(test.expected) {
				t.Fatalf("delays = %v", delays)
			}
			for index := range delays {
				if delays[index] != test.expected[index] {
					t.Fatalf("delays = %v, want %v", delays, test.expected)
				}
			}
			if test.errorDelay && stderr.Len() == 0 {
				t.Fatal("control-plane errors were not reported")
			}
		})
	}
}

func TestResolveAgentWorkTokenFromProtectedFile(t *testing.T) {
	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "agent.token")
	if err := os.WriteFile(tokenPath, []byte("file-work-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := resolveAgentWorkToken("", "GITSSS_AGENT_WORK_TOKEN", tokenPath)
	if err != nil || token != "file-work-token" {
		t.Fatalf("token = %q, %v", token, err)
	}
	if _, err := resolveAgentWorkToken("", "GITSSS_AGENT_WORK_TOKEN", "relative.token"); err == nil {
		t.Fatal("relative token file was accepted")
	}
	largePath := filepath.Join(directory, "large.token")
	if err := os.WriteFile(largePath, []byte(strings.Repeat("x", 4097)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAgentWorkToken("", "GITSSS_AGENT_WORK_TOKEN", largePath); err == nil {
		t.Fatal("oversized token file was accepted")
	}
	linkPath := filepath.Join(directory, "link.token")
	if err := os.Symlink(tokenPath, linkPath); err == nil {
		if _, err := resolveAgentWorkToken("", "GITSSS_AGENT_WORK_TOKEN", linkPath); err == nil {
			t.Fatal("symlink token file was accepted")
		}
	}
}

func TestCheckAgentHealthUsesWorkTokenWithoutMutatingQueue(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.URL.Path != "/api/v1/agent/health" || request.Header.Get("Authorization") != "Bearer work-token" {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		writeWorkerJSON(t, response, map[string]string{"status": "ok"})
	}))
	defer server.Close()
	if err := checkAgentHealth(context.Background(), server.Client(), server.URL, "work-token"); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
}

func writeWorkerJSON(t *testing.T, response http.ResponseWriter, value any) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(value); err != nil {
		t.Error(err)
	}
}
