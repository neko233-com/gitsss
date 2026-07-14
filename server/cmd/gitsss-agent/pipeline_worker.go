// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/neko233-com/gitsss/server/internal/pipeline"
	"github.com/neko233-com/gitsss/server/internal/state"
)

type pipelineExecutor func(context.Context, pipeline.Claim, string, string, string, *http.Client) error

type pendingPipelineCompletion struct {
	runID      string
	succeeded  bool
	failure    string
	executeErr error
}

type pipelineCycleOutcome struct {
	Found      bool
	Completed  pipeline.Run
	ExecuteErr error
}

type pipelineWorker struct {
	serverURL string
	token     string
	runnerID  string
	workspace string
	client    *http.Client
	execute   pipelineExecutor
	pending   *pendingPipelineCompletion
}

func pipelineWorkOnce(args []string) {
	options, err := parsePipelineWorkOptions("pipeline-work-once", args, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	worker := newPipelineWorker(options.serverURL, options.token, options.runnerID, options.workspace, nil, nil)
	outcome, err := worker.Cycle(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !outcome.Found {
		fmt.Println("no pipeline run available")
		return
	}
	fmt.Printf("pipeline %s: %s\n", outcome.Completed.ID, outcome.Completed.Status)
	if outcome.ExecuteErr != nil {
		fmt.Fprintln(os.Stderr, outcome.ExecuteErr)
		os.Exit(1)
	}
}

func pipelineWork(args []string) {
	options, err := parsePipelineWorkOptions("pipeline-work", args, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	worker := newPipelineWorker(options.serverURL, options.token, options.runnerID, options.workspace, nil, nil)
	fmt.Printf("gitsss Runner %s polling %s\n", options.runnerID, options.serverURL)
	if err := runPipelineLoop(ctx, worker, options.idleMin, options.idleMax, options.errorMax, waitForAgent, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type pipelineWorkOptions struct {
	serverURL string
	token     string
	runnerID  string
	workspace string
	idleMin   time.Duration
	idleMax   time.Duration
	errorMax  time.Duration
}

func parsePipelineWorkOptions(name string, args []string, loop bool) (pipelineWorkOptions, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	serverURL := flags.String("server", "", "gitsss API base URL")
	tokenEnv := flags.String("token-env", "GITSSS_RUNNER_TOKEN", "environment variable containing the Runner Token")
	tokenFile := flags.String("token-file", "", "absolute path to a protected Runner Token file")
	runnerID := flags.String("id", "", "Runner ID matching the Token ID")
	workspace := flags.String("workspace", "", "absolute Runner workspace")
	idleMin := flags.Duration("idle-min", time.Second, "minimum empty-queue delay")
	idleMax := flags.Duration("idle-max", 30*time.Second, "maximum empty-queue delay")
	errorMax := flags.Duration("error-max", 2*time.Minute, "maximum control-plane error delay")
	if err := flags.Parse(args); err != nil {
		return pipelineWorkOptions{}, err
	}
	token, err := resolveAgentWorkToken("", *tokenEnv, *tokenFile)
	if err != nil || strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*runnerID) == "" {
		return pipelineWorkOptions{}, errors.New("--server, --id and a valid --token-env or --token-file are required")
	}
	parsedURL, err := url.Parse(*serverURL)
	if err != nil || parsedURL.Host == "" || parsedURL.Scheme != "http" && parsedURL.Scheme != "https" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return pipelineWorkOptions{}, errors.New("--server must be an HTTP(S) base URL without credentials, query or fragment")
	}
	workspacePath := strings.TrimSpace(*workspace)
	if workspacePath == "" {
		cache, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			return pipelineWorkOptions{}, cacheErr
		}
		workspacePath = filepath.Join(cache, "gitsss", "runner")
	}
	if !filepath.IsAbs(workspacePath) {
		return pipelineWorkOptions{}, errors.New("--workspace must be absolute")
	}
	if loop && (*idleMin <= 0 || *idleMax < *idleMin || *idleMax > 5*time.Minute || *errorMax < *idleMin || *errorMax > 15*time.Minute) {
		return pipelineWorkOptions{}, errors.New("poll delays must be positive, ordered, and capped at 5m idle / 15m error")
	}
	return pipelineWorkOptions{
		serverURL: strings.TrimRight(parsedURL.String(), "/"), token: token, runnerID: *runnerID,
		workspace: filepath.Clean(workspacePath), idleMin: *idleMin, idleMax: *idleMax, errorMax: *errorMax,
	}, nil
}

func newPipelineWorker(serverURL, token, runnerID, workspace string, client *http.Client, execute pipelineExecutor) *pipelineWorker {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	if execute == nil {
		execute = executePipeline
	}
	return &pipelineWorker{
		serverURL: strings.TrimRight(serverURL, "/"), token: token, runnerID: runnerID,
		workspace: workspace, client: client, execute: execute,
	}
}

func (worker *pipelineWorker) Cycle(ctx context.Context) (pipelineCycleOutcome, error) {
	if worker.pending != nil {
		return worker.flushPending(ctx)
	}
	claim, found, err := claimPipeline(ctx, worker.client, worker.serverURL, worker.token, worker.runnerID)
	if err != nil || !found {
		return pipelineCycleOutcome{}, err
	}
	executionErr := worker.execute(ctx, claim, worker.serverURL, worker.token, worker.workspace, worker.client)
	failure := ""
	if executionErr != nil {
		failure = singleLineError(executionErr)
	}
	worker.pending = &pendingPipelineCompletion{
		runID: claim.Run.ID, succeeded: executionErr == nil, failure: failure, executeErr: executionErr,
	}
	return worker.flushPending(ctx)
}

func (worker *pipelineWorker) flushPending(ctx context.Context) (pipelineCycleOutcome, error) {
	pending := worker.pending
	completed, err := completePipeline(ctx, worker.client, worker.serverURL, worker.token, worker.runnerID, pending.runID, pending.succeeded, pending.failure)
	if err != nil {
		return pipelineCycleOutcome{Found: true}, err
	}
	worker.pending = nil
	return pipelineCycleOutcome{Found: true, Completed: completed, ExecuteErr: pending.executeErr}, nil
}

func executePipeline(ctx context.Context, claim pipeline.Claim, serverURL, token, workspace string, client *http.Client) error {
	local, ok := pipeline.TemplateByID(claim.Run.TemplateID)
	if !ok || !reflect.DeepEqual(local, claim.Template) {
		return errors.New("pipeline template does not match the built-in Runner allowlist")
	}
	if claim.Run.Status != pipeline.StatusRunning || claim.Run.RunnerID == "" {
		return errors.New("pipeline claim is not assigned")
	}
	if err := prepareWorkspaceRoot(workspace); err != nil {
		return err
	}
	runRoot := filepath.Join(workspace, claim.Run.ID)
	if filepath.Dir(runRoot) != workspace {
		return errors.New("pipeline workspace escaped its root")
	}
	if err := os.Mkdir(runRoot, 0o700); err != nil {
		return fmt.Errorf("create pipeline workspace: %w", err)
	}
	if err := state.ProtectDirectory(runRoot); err != nil {
		_ = os.RemoveAll(runRoot)
		return err
	}
	defer os.RemoveAll(runRoot)
	source := filepath.Join(runRoot, "source")
	if err := clonePipelineRepository(ctx, serverURL, token, claim.Run.RepositoryID, claim.Run.Commit, source); err != nil {
		return heartbeatFailure(ctx, client, serverURL, token, claim.Run, "checkout failed: "+singleLineError(err))
	}
	for _, step := range local.Steps {
		if err := runPipelineStep(ctx, client, serverURL, token, claim.Run, source, step); err != nil {
			return err
		}
	}
	return nil
}

func prepareWorkspaceRoot(workspace string) error {
	if !filepath.IsAbs(workspace) {
		return errors.New("pipeline workspace must be absolute")
	}
	info, err := os.Lstat(workspace)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(workspace, 0o700); err != nil {
			return err
		}
		return state.ProtectDirectory(workspace)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("pipeline workspace must be a real directory")
	}
	return state.ProtectDirectory(workspace)
}

func clonePipelineRepository(ctx context.Context, serverURL, token, repositoryID, commit, target string) error {
	cloneURL := strings.TrimRight(serverURL, "/") + "/git/" + url.PathEscape(repositoryID) + ".git"
	command := exec.CommandContext(ctx, "git", "clone", "--no-checkout", "--filter=blob:none", cloneURL, target)
	command.Env = append(safeRunnerEnvironment(),
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Bearer "+token,
		"GIT_CONFIG_KEY_1=protocol.version", "GIT_CONFIG_VALUE_1=2",
	)
	if output, err := boundedCombinedOutput(command, 64<<10); err != nil {
		return fmt.Errorf("clone repository: %w: %s", err, strings.TrimSpace(output))
	}
	checkout := exec.CommandContext(ctx, "git", "-C", target, "checkout", "--detach", commit)
	checkout.Env = safeRunnerEnvironment()
	if output, err := boundedCombinedOutput(checkout, 64<<10); err != nil {
		return fmt.Errorf("checkout commit: %w: %s", err, strings.TrimSpace(output))
	}
	return nil
}

func runPipelineStep(ctx context.Context, client *http.Client, serverURL, token string, run pipeline.Run, source string, step pipeline.Step) error {
	directory, err := safeStepDirectory(source, step.WorkingDirectory)
	if err != nil {
		return err
	}
	stepContext, cancel := context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
	defer cancel()
	output := &synchronizedLimitedBuffer{limit: 64 << 10}
	command := exec.CommandContext(stepContext, step.Command[0], step.Command[1:]...)
	command.Dir = directory
	command.Env = append(safeRunnerEnvironment(), "CI=true", "GIT_TERMINAL_PROMPT=0")
	command.Stdout = output
	command.Stderr = output
	done := make(chan error, 1)
	if err := command.Start(); err != nil {
		return heartbeatFailure(ctx, client, serverURL, token, run, fmt.Sprintf("[%s] start failed: %v\n", step.ID, err))
	}
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case commandErr := <-done:
			log := fmt.Sprintf("\n[%s] %s\n%s", step.ID, step.Name, output.String())
			if heartbeatErr := sendPipelineLog(ctx, client, serverURL, token, run, log); heartbeatErr != nil {
				return heartbeatErr
			}
			if commandErr != nil {
				return fmt.Errorf("pipeline step %s failed: %w", step.ID, commandErr)
			}
			return nil
		case <-ticker.C:
			if err := heartbeatPipeline(ctx, client, serverURL, token, run.ID, run.RunnerID, ""); err != nil {
				cancel()
				<-done
				return err
			}
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}
}

func safeStepDirectory(source, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("pipeline step working directory is invalid")
	}
	root, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	relativeToRoot, err := filepath.Rel(root, resolved)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(os.PathSeparator)) {
		return "", errors.New("pipeline step working directory escaped the checkout")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("pipeline step working directory is not a directory")
	}
	return resolved, nil
}

func safeRunnerEnvironment() []string {
	allowed := map[string]struct{}{
		"PATH": {}, "Path": {}, "SYSTEMROOT": {}, "SystemRoot": {}, "COMSPEC": {}, "ComSpec": {},
		"TEMP": {}, "TMP": {}, "TMPDIR": {}, "HOME": {}, "USERPROFILE": {}, "LOCALAPPDATA": {},
		"APPDATA": {}, "PROGRAMDATA": {}, "GOROOT": {}, "GOPATH": {}, "GOCACHE": {}, "GOMODCACHE": {},
		"PNPM_HOME": {}, "NPM_CONFIG_CACHE": {}, "HTTPS_PROXY": {}, "HTTP_PROXY": {}, "NO_PROXY": {},
	}
	environment := make([]string, 0, len(allowed)+2)
	for _, item := range os.Environ() {
		name, _, ok := strings.Cut(item, "=")
		if _, permitted := allowed[name]; ok && permitted {
			environment = append(environment, item)
		}
	}
	if runtime.GOOS == "windows" {
		environment = append(environment, "GIT_CONFIG_NOSYSTEM=1")
	}
	return environment
}

func claimPipeline(ctx context.Context, client *http.Client, serverURL, token, runnerID string) (pipeline.Claim, bool, error) {
	var claim pipeline.Claim
	status, err := pipelineJSONRequest(ctx, client, http.MethodPost, serverURL+"/api/v1/pipelines/runs/claim", token, map[string]string{"runner_id": runnerID}, &claim)
	if err != nil {
		return pipeline.Claim{}, false, err
	}
	if status == http.StatusNoContent {
		return pipeline.Claim{}, false, nil
	}
	if status != http.StatusOK {
		return pipeline.Claim{}, false, fmt.Errorf("pipeline claim returned %s", http.StatusText(status))
	}
	return claim, true, nil
}

func heartbeatPipeline(ctx context.Context, client *http.Client, serverURL, token, runID, runnerID, log string) error {
	status, err := pipelineJSONRequest(ctx, client, http.MethodPost, serverURL+"/api/v1/pipelines/runs/"+url.PathEscape(runID)+"/heartbeat", token, map[string]string{"runner_id": runnerID, "log": log}, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("pipeline heartbeat returned %s", http.StatusText(status))
	}
	return nil
}

func completePipeline(ctx context.Context, client *http.Client, serverURL, token, runnerID, runID string, succeeded bool, failure string) (pipeline.Run, error) {
	var completed pipeline.Run
	payload := struct {
		RunnerID  string `json:"runner_id"`
		Succeeded bool   `json:"succeeded"`
		Error     string `json:"error"`
	}{RunnerID: runnerID, Succeeded: succeeded, Error: failure}
	status, err := pipelineJSONRequest(ctx, client, http.MethodPost, serverURL+"/api/v1/pipelines/runs/"+url.PathEscape(runID)+"/complete", token, payload, &completed)
	if err != nil {
		return pipeline.Run{}, err
	}
	if status != http.StatusOK {
		return pipeline.Run{}, fmt.Errorf("pipeline completion returned %s", http.StatusText(status))
	}
	return completed, nil
}

func sendPipelineLog(ctx context.Context, client *http.Client, serverURL, token string, run pipeline.Run, log string) error {
	for len(log) > 0 {
		length := min(len(log), 32<<10)
		if err := heartbeatPipeline(ctx, client, serverURL, token, run.ID, run.RunnerID, log[:length]); err != nil {
			return err
		}
		log = log[length:]
	}
	return nil
}

func heartbeatFailure(ctx context.Context, client *http.Client, serverURL, token string, run pipeline.Run, message string) error {
	if err := sendPipelineLog(ctx, client, serverURL, token, run, message+"\n"); err != nil {
		return errors.Join(errors.New(message), err)
	}
	return errors.New(message)
}

func pipelineJSONRequest(ctx context.Context, client *http.Client, method, endpoint, token string, payload, result any) (int, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(content))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return response.StatusCode, fmt.Errorf("%s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if result != nil && response.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}

func boundedCombinedOutput(command *exec.Cmd, limit int) (string, error) {
	buffer := &synchronizedLimitedBuffer{limit: limit}
	command.Stdout = buffer
	command.Stderr = buffer
	err := command.Run()
	return buffer.String(), err
}

type synchronizedLimitedBuffer struct {
	mu        sync.Mutex
	content   bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *synchronizedLimitedBuffer) Write(content []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	remaining := buffer.limit - buffer.content.Len()
	if remaining > 0 {
		_, _ = buffer.content.Write(content[:min(len(content), remaining)])
	}
	if len(content) > remaining {
		buffer.truncated = true
	}
	return len(content), nil
}

func (buffer *synchronizedLimitedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	result := buffer.content.String()
	if buffer.truncated {
		result += "\n[output truncated]\n"
	}
	return result
}

func singleLineError(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}

func runPipelineLoop(ctx context.Context, worker *pipelineWorker, idleMin, idleMax, errorMax time.Duration, wait agentWaitFunc, stdout, stderr io.Writer) error {
	idleDelay := idleMin
	errorDelay := idleMin
	for {
		outcome, err := worker.Cycle(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			fmt.Fprintf(stderr, "Runner control-plane error: %v; retrying in %s\n", err, errorDelay)
			if !wait(ctx, errorDelay) {
				return nil
			}
			errorDelay = boundedAgentDelay(errorDelay, errorMax)
			continue
		}
		errorDelay = idleMin
		if outcome.Found {
			idleDelay = idleMin
			fmt.Fprintf(stdout, "pipeline %s: %s\n", outcome.Completed.ID, outcome.Completed.Status)
			if outcome.ExecuteErr != nil {
				fmt.Fprintf(stderr, "pipeline %s failed: %v\n", outcome.Completed.ID, outcome.ExecuteErr)
			}
			continue
		}
		if !wait(ctx, idleDelay) {
			return nil
		}
		idleDelay = boundedAgentDelay(idleDelay, idleMax)
	}
}
