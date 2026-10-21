// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hookrelay runs the existing hook API without initializing the web application.
package hookrelay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json" //nolint:depguard // The standalone hook must avoid application package initialization.
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	Protocol = "gitsss-hook/2"
	// DelegateChanged asks the shell dispatcher to execute the original script.
	DelegateChanged = 75
	EnvPrefix       = "GITSSS_HOOK_"
)

type Options struct {
	OldCommitIDs                    []string
	NewCommitIDs                    []string
	RefFullNames                    []string
	UserID                          int64
	UserName                        string
	GitObjectDirectory              string
	GitAlternativeObjectDirectories string
	GitQuarantinePath               string
	GitPushOptions                  map[string]string
	PullRequestID                   int64
	PushTrigger                     string
	DeployKeyID                     int64
	IsWiki                          bool
	ActionsTaskID                   int64
}

type BranchResult struct {
	Message bool
	Create  bool
	Branch  string
	URL     string
}

// LocalURL limits the credential-bearing relay to the existing local HTTP API.
func LocalURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
}

func PublicEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	original := ""
	for _, item := range environment {
		if value, ok := strings.CutPrefix(item, EnvPrefix+"ORIGINAL_DIR="); ok {
			original = value
		}
	}
	for _, item := range environment {
		if !strings.HasPrefix(item, EnvPrefix) {
			if original != "" && strings.HasPrefix(item, "GIT_CONFIG_PARAMETERS=") {
				// Git passes -c options to hooks; nested Git must use the original hooks.
				value := strings.ReplaceAll("core.hooksPath="+original, "'", "'\\''")
				item += " '" + value + "'"
			}
			result = append(result, item)
		}
	}
	return result
}

func DelegateMatches(mode string, getenv func(string) string) bool {
	expected := getenv(EnvPrefix + "SHA_" + strings.ToUpper(strings.ReplaceAll(mode, "-", "_")))
	if len(expected) != sha256.Size*2 {
		return false
	}
	path := filepath.Join(getenv(EnvPrefix+"ORIGINAL_DIR"), mode+".d", "gitea")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return false
	}
	data, err := os.ReadFile(path)
	digest := sha256.Sum256(data)
	return err == nil && hex.EncodeToString(digest[:]) == expected
}

func optionsFromEnvironment(mode string, getenv func(string) string) Options {
	integer := func(key string) int64 { n, _ := strconv.ParseInt(getenv(key), 10, 64); return n }
	wiki, _ := strconv.ParseBool(getenv("GITEA_REPO_IS_WIKI"))
	options := Options{
		UserID: integer("GITEA_PUSHER_ID"), PullRequestID: integer("GITEA_PR_ID"), IsWiki: wiki,
		GitObjectDirectory: getenv("GIT_OBJECT_DIRECTORY"), GitAlternativeObjectDirectories: getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"),
		GitQuarantinePath: getenv("GIT_QUARANTINE_PATH"), GitPushOptions: make(map[string]string),
	}
	count, _ := strconv.Atoi(getenv("GIT_PUSH_OPTION_COUNT"))
	for index := range count {
		parts := strings.SplitN(getenv("GIT_PUSH_OPTION_"+strconv.Itoa(index)), "=", 2)
		if len(parts) == 2 {
			options.GitPushOptions[parts[0]] = parts[1]
		}
	}
	if mode == "pre-receive" {
		options.DeployKeyID = integer("GITEA_DEPLOY_KEY_ID")
		options.ActionsTaskID = integer("GITEA_ACTIONS_TASK_ID")
	} else {
		options.UserName = getenv("GITEA_PUSHER_NAME")
		options.PushTrigger = getenv("GITEA_PUSH_TRIGGER")
	}
	return options
}

func request(ctx context.Context, client *http.Client, mode string, options Options, getenv func(string) string) ([]BranchResult, error) {
	base := getenv(EnvPrefix + "URL")
	token := getenv(EnvPrefix + "TOKEN")
	if !LocalURL(base) || token == "" {
		return nil, errors.New("internal hook transport is unavailable")
	}
	endpoint := strings.TrimRight(base, "/") + "/api/internal/hook/" + mode + "/" + url.PathEscape(getenv("GITEA_REPO_USER_NAME")) + "/" + url.PathEscape(getenv("GITEA_REPO_NAME"))
	body, err := json.Marshal(options)
	if err != nil {
		return nil, errors.New("internal hook request could not be encoded")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("internal hook request is invalid")
	}
	req.Header.Set("X-Gitea-Internal-Auth", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	clientIP := "127.0.0.1"
	if fields := strings.Fields(getenv("SSH_CONNECTION")); len(fields) > 0 && net.ParseIP(fields[0]) != nil {
		clientIP = fields[0]
	}
	req.Header.Set("X-Real-IP", clientIP)
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("internal server connection error")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
	if err != nil || len(data) > 16*1024*1024 {
		return nil, errors.New("internal server response reading failed")
	}
	var result struct {
		Err     string `json:"err"`
		UserMsg string `json:"user_msg"`
		Results []BranchResult
	}
	decodeErr := json.Unmarshal(data, &result)
	if response.StatusCode/100 != 2 || result.Err != "" {
		if result.UserMsg != "" {
			return nil, fmt.Errorf("%s", result.UserMsg)
		}
		return nil, errors.New("internal server error (no message for end users)")
	}
	if mode == "post-receive" && decodeErr != nil {
		return nil, errors.New("internal server response decoding failed")
	}
	return result.Results, nil
}

func printResults(output io.Writer, results []BranchResult) {
	for _, result := range results {
		if !result.Message {
			continue
		}
		fmt.Fprintln(output)
		if result.Create {
			fmt.Fprintf(output, "Create a new pull request for '%s':\n", result.Branch)
		} else {
			fmt.Fprintln(output, "Visit the existing pull request:")
		}
		fmt.Fprintf(output, "  %s\n\n", result.URL)
	}
}

type progressWriter struct {
	output io.Writer
	start  time.Time
	delay  time.Duration
	buffer bytes.Buffer
	ready  bool
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	if writer.ready || time.Since(writer.start) >= writer.delay {
		if !writer.ready {
			if _, err := writer.buffer.WriteTo(writer.output); err != nil {
				return 0, err
			}
			writer.ready = true
		}
		return writer.output.Write(data)
	}
	return writer.buffer.Write(data)
}

func (writer *progressWriter) Close() {
	if !writer.ready && time.Since(writer.start) >= writer.delay {
		_, _ = writer.buffer.WriteTo(writer.output)
	}
}

// Run receives the same ref batches and environment as the original managed hook.
func Run(ctx context.Context, mode string, args []string, input io.Reader, output, errorOutput io.Writer, getenv func(string) string) int {
	return run(ctx, mode, args, input, output, errorOutput, getenv, func(ctx context.Context) error {
		command := exec.CommandContext(ctx, getenv(EnvPrefix+"GIT"), "update-server-info")
		command.Env = PublicEnvironment(os.Environ())
		command.Stderr = errorOutput
		return command.Run()
	})
}

func run(ctx context.Context, mode string, args []string, input io.Reader, output, errorOutput io.Writer, getenv func(string) string, updateServerInfo func(context.Context) error) int {
	if mode != "pre-receive" && mode != "post-receive" && mode != "update" || !DelegateMatches(mode, getenv) {
		return DelegateChanged
	}
	internal, _ := strconv.ParseBool(getenv("GITEA_INTERNAL_PUSH"))
	if mode == "update" {
		if !internal && len(args) >= 3 {
			ref := args[len(args)-3]
			if strings.HasPrefix(ref, "refs/pull/") && strings.Contains(ref[len("refs/pull/"):], "/") {
				return 1
			}
		}
		return 0
	}
	fail := func(err error) int {
		fmt.Fprintf(errorOutput, "\nerror:\nerror: %s\nerror:\n", err)
		return 1
	}
	if mode == "post-receive" {
		if err := updateServerInfo(ctx); err != nil {
			return fail(errors.New("failed to call 'git update-server-info'"))
		}
	}
	if internal {
		return 0
	}
	if getenv("SSH_ORIGINAL_COMMAND") == "" {
		return fail(errors.New("rejecting changes as gitsss environment not set"))
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	options := optionsFromEnvironment(mode, getenv)
	results := make([]BranchResult, 0)
	delay, _ := time.ParseDuration(getenv(EnvPrefix + "VERBOSE_DELAY"))
	progressOutput := io.Discard
	if getenv(EnvPrefix+"VERBOSE") == "true" {
		writer := &progressWriter{output: output, start: time.Now(), delay: delay}
		defer writer.Close()
		progressOutput = writer
	}
	defer func() { printResults(errorOutput, results) }()
	progress := func(format string, args ...any) {
		fmt.Fprintf(progressOutput, format, args...)
	}
	flush := func() error {
		if len(options.RefFullNames) == 0 {
			return nil
		}
		if mode == "pre-receive" {
			progress(" Checking %d references\n", len(options.RefFullNames))
		} else {
			progress(" Processing %d references\n", len(options.RefFullNames))
		}
		batch, err := request(ctx, client, mode, options, getenv)
		if err != nil {
			return err
		}
		results = append(results, batch...)
		options.OldCommitIDs = options.OldCommitIDs[:0]
		options.NewCommitIDs = options.NewCommitIDs[:0]
		options.RefFullNames = options.RefFullNames[:0]
		return nil
	}
	scanner := bufio.NewScanner(input)
	total := 0
	for scanner.Scan() {
		if options.IsWiki {
			continue
		}
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		total++
		if mode == "pre-receive" && getenv(EnvPrefix+"PROC_RECEIVE") != "true" && !strings.HasPrefix(fields[2], "refs/heads/") && !strings.HasPrefix(fields[2], "refs/tags/") {
			progress(".")
			continue
		}
		options.OldCommitIDs = append(options.OldCommitIDs, fields[0])
		options.NewCommitIDs = append(options.NewCommitIDs, fields[1])
		options.RefFullNames = append(options.RefFullNames, fields[2])
		if mode == "pre-receive" {
			progress("*")
		} else {
			progress(".")
		}
		if len(options.RefFullNames) == 500 {
			if err := flush(); err != nil {
				return fail(err)
			}
		}
	}
	if scanner.Err() != nil {
		return fail(errors.New("hook failed: stdin read error"))
	}
	if err := flush(); err != nil {
		return fail(err)
	}
	if mode == "pre-receive" {
		progress("Checked %d references in total\n", total)
	}
	return 0
}
