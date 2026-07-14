// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
)

type agentJobExecutor func(agentjob.Job, string) (json.RawMessage, error)

type pendingAgentCompletion struct {
	jobID        string
	result       json.RawMessage
	failure      string
	executionErr error
}

type agentCycleOutcome struct {
	Found        bool
	Completed    agentjob.Job
	ExecutionErr error
}

type agentWorker struct {
	serverURL string
	token     string
	agentID   string
	client    *http.Client
	execute   agentJobExecutor
	pending   *pendingAgentCompletion
}

func health(args []string) {
	flags := flag.NewFlagSet("health", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	tokenEnv := flags.String("token-env", "GITSSS_AGENT_WORK_TOKEN", "environment variable containing the Agent work token")
	tokenFile := flags.String("token-file", "", "absolute path to a protected Agent work token file")
	flags.Parse(args)
	token, err := resolveAgentWorkToken("", *tokenEnv, *tokenFile)
	if strings.TrimSpace(*serverURL) == "" || err != nil {
		fmt.Fprintln(os.Stderr, "--server and a valid --token-env or --token-file are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := checkAgentHealth(ctx, &http.Client{Timeout: 15 * time.Second}, *serverURL, token); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("gitsss Agent control plane is healthy")
}

func checkAgentHealth(ctx context.Context, client *http.Client, serverURL, token string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(serverURL, "/")+"/api/v1/agent/health", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("Agent health: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result struct {
		Status string `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<10))
	if err := decoder.Decode(&result); err != nil || result.Status != "ok" {
		return fmt.Errorf("Agent health returned an invalid response")
	}
	return nil
}

func newAgentWorker(serverURL, token, agentID string, client *http.Client, execute agentJobExecutor) *agentWorker {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if execute == nil {
		execute = executeAgentJob
	}
	return &agentWorker{
		serverURL: strings.TrimRight(serverURL, "/"), token: token, agentID: agentID,
		client: client, execute: execute,
	}
}

func (worker *agentWorker) Cycle(ctx context.Context) (agentCycleOutcome, error) {
	if worker.pending != nil {
		return worker.flushPending(ctx)
	}
	job, found, err := claimAgentJobContext(ctx, worker.client, worker.serverURL, worker.token, worker.agentID)
	if err != nil || !found {
		return agentCycleOutcome{}, err
	}
	result, executionErr := worker.execute(job, worker.agentID)
	failure := ""
	if executionErr != nil {
		failure = executionErr.Error()
	}
	worker.pending = &pendingAgentCompletion{
		jobID: job.ID, result: result, failure: failure, executionErr: executionErr,
	}
	return worker.flushPending(ctx)
}

func (worker *agentWorker) flushPending(ctx context.Context) (agentCycleOutcome, error) {
	pending := worker.pending
	completed, err := completeAgentJobContext(
		ctx, worker.client, worker.serverURL, worker.token, worker.agentID,
		pending.jobID, pending.result, pending.failure,
	)
	if err != nil {
		return agentCycleOutcome{Found: true}, err
	}
	worker.pending = nil
	return agentCycleOutcome{Found: true, Completed: completed, ExecutionErr: pending.executionErr}, nil
}

func runWorkOnce(args []string) {
	flags := flag.NewFlagSet("work-once", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "Agent work token; prefer --token-env")
	tokenEnv := flags.String("token-env", "GITSSS_AGENT_WORK_TOKEN", "environment variable containing the Agent work token")
	tokenFile := flags.String("token-file", "", "absolute path to a protected Agent work token file")
	agentID := flags.String("id", "", "stable Agent device ID")
	flags.Parse(args)
	tokenValue, err := resolveAgentWorkToken(*token, *tokenEnv, *tokenFile)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*agentID) == "" || err != nil {
		fmt.Fprintln(os.Stderr, "--server, --id and an Agent work token are required")
		os.Exit(2)
	}
	worker := newAgentWorker(*serverURL, tokenValue, *agentID, nil, nil)
	outcome, err := worker.Cycle(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !outcome.Found {
		fmt.Println("no Agent job available")
		return
	}
	fmt.Printf("Agent job %s: %s\n", outcome.Completed.ID, outcome.Completed.Status)
	if outcome.ExecutionErr != nil {
		fmt.Fprintln(os.Stderr, outcome.ExecutionErr)
		os.Exit(1)
	}
}

func work(args []string) {
	flags := flag.NewFlagSet("work", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	tokenEnv := flags.String("token-env", "GITSSS_AGENT_WORK_TOKEN", "environment variable containing the Agent work token")
	tokenFile := flags.String("token-file", "", "absolute path to a protected Agent work token file")
	agentID := flags.String("id", "", "stable Agent device ID")
	idleMin := flags.Duration("idle-min", time.Second, "minimum delay when the queue is empty")
	idleMax := flags.Duration("idle-max", 30*time.Second, "maximum delay when the queue stays empty")
	errorMax := flags.Duration("error-max", 2*time.Minute, "maximum delay after control-plane errors")
	flags.Parse(args)
	tokenValue, tokenErr := resolveAgentWorkToken("", *tokenEnv, *tokenFile)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*agentID) == "" || tokenErr != nil {
		fmt.Fprintln(os.Stderr, "--server, --id and a valid --token-env or --token-file are required")
		os.Exit(2)
	}
	if *idleMin <= 0 || *idleMax < *idleMin || *idleMax > 5*time.Minute || *errorMax < *idleMin || *errorMax > 15*time.Minute {
		fmt.Fprintln(os.Stderr, "poll delays must be positive, ordered, and capped at 5m idle / 15m error")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	worker := newAgentWorker(*serverURL, tokenValue, *agentID, nil, nil)
	fmt.Printf("gitsss Agent %s polling %s\n", *agentID, strings.TrimRight(*serverURL, "/"))
	if err := runAgentLoop(ctx, worker, *idleMin, *idleMax, *errorMax, waitForAgent, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveAgentWorkToken(explicit, environmentName, filePath string) (string, error) {
	if token := strings.TrimSpace(explicit); token != "" {
		return token, nil
	}
	if strings.TrimSpace(filePath) != "" {
		return readAgentWorkTokenFile(filePath)
	}
	if !validAgentEnvironmentName(environmentName) {
		return "", fmt.Errorf("invalid Agent work token environment")
	}
	token := strings.TrimSpace(os.Getenv(environmentName))
	if token == "" {
		return "", fmt.Errorf("%s is empty", environmentName)
	}
	return token, nil
}

func readAgentWorkTokenFile(filePath string) (string, error) {
	if !filepath.IsAbs(filePath) {
		return "", fmt.Errorf("Agent work token file must use an absolute path")
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4096 {
		return "", fmt.Errorf("Agent work token file must be a regular file up to 4096 bytes")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("Agent work token file must not be accessible by group or others")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	currentInfo, err := os.Lstat(filePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		return "", fmt.Errorf("Agent work token file changed while opening")
	}
	content, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(content))
	if token == "" || len(content) > 4096 {
		return "", fmt.Errorf("Agent work token file is empty or too large")
	}
	return token, nil
}

type agentWaitFunc func(context.Context, time.Duration) bool

func runAgentLoop(ctx context.Context, worker *agentWorker, idleMin, idleMax, errorMax time.Duration, wait agentWaitFunc, stdout, stderr io.Writer) error {
	idleDelay := idleMin
	errorDelay := idleMin
	for {
		outcome, err := worker.Cycle(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			fmt.Fprintf(stderr, "Agent control-plane error: %v; retrying in %s\n", err, errorDelay)
			if !wait(ctx, errorDelay) {
				return nil
			}
			errorDelay = boundedAgentDelay(errorDelay, errorMax)
			continue
		}
		errorDelay = idleMin
		if outcome.Found {
			idleDelay = idleMin
			fmt.Fprintf(stdout, "Agent job %s: %s\n", outcome.Completed.ID, outcome.Completed.Status)
			if outcome.ExecutionErr != nil {
				fmt.Fprintf(stderr, "Agent job %s failed: %v\n", outcome.Completed.ID, outcome.ExecutionErr)
			}
			continue
		}
		if !wait(ctx, idleDelay) {
			return nil
		}
		idleDelay = boundedAgentDelay(idleDelay, idleMax)
	}
}

func boundedAgentDelay(current, maximum time.Duration) time.Duration {
	if current >= maximum || current > maximum/2 {
		return maximum
	}
	return current * 2
}

func waitForAgent(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
