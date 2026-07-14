// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/backup"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "probe":
		probe(os.Args[2:])
	case "runner-report":
		runnerReport(os.Args[2:])
	case "assess":
		assess(os.Args[2:])
	case "backup-export":
		backupExport(os.Args[2:])
	case "backup-verify":
		backupVerify(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: gitsss-agent <probe|runner-report|assess|backup-export|backup-verify> [options]")
}

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func assess(args []string) {
	flags := flag.NewFlagSet("assess", flag.ExitOnError)
	var configPaths stringList
	flags.Var(&configPaths, "config", "Gitea app.ini path; may be repeated")
	workPath := flags.String("work-path", "", "Gitea application work path for relative settings")
	serverURL := flags.String("server", "", "gitsss API base URL; omit for local JSON output")
	token := flags.String("token", "", "agent token; required with --server")
	agentID := flags.String("agent-id", "", "stable agent device ID")
	version := flags.String("gitea-version", "", "detected Gitea version when known")
	maxEntries := flags.Int("max-entries", 10_000, "maximum files scanned per asset category")
	flags.Parse(args)
	if strings.TrimSpace(*agentID) == "" {
		fmt.Fprintln(os.Stderr, "--agent-id is required")
		os.Exit(2)
	}
	if (strings.TrimSpace(*serverURL) == "") != (strings.TrimSpace(*token) == "") {
		fmt.Fprintln(os.Stderr, "--server and --token must be provided together")
		os.Exit(2)
	}
	if len(configPaths) == 0 {
		configPaths = takeover.DiscoverConfigPaths()
		if len(configPaths) == 0 {
			fmt.Fprintln(os.Stderr, "no Gitea app.ini found; pass --config explicitly")
			os.Exit(1)
		}
	}

	reports := make([]takeover.Report, 0, len(configPaths))
	for _, configPath := range configPaths {
		report, err := takeover.Assess(takeover.Options{
			AgentID: *agentID, ConfigPath: configPath, WorkPath: *workPath,
			GiteaVersion: *version, MaxEntries: *maxEntries,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "assess %s: %v\n", configPath, err)
			os.Exit(1)
		}
		reports = append(reports, report)
	}

	if strings.TrimSpace(*serverURL) == "" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(reports); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	for _, report := range reports {
		if err := uploadAssessment(*serverURL, *token, report); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("assessment uploaded: %s (%d findings, %d asset categories)\n", report.ID, len(report.Findings), len(report.Assets))
	}
}

func uploadAssessment(serverURL, token string, report takeover.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(serverURL, "/")+"/api/v1/takeover/assessments", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return fmt.Errorf("assessment upload failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func backupExport(args []string) {
	flags := flag.NewFlagSet("backup-export", flag.ExitOnError)
	configPath := flags.String("config", "", "Gitea app.ini path; auto-discovered when exactly one exists")
	workPath := flags.String("work-path", "", "Gitea application work path for relative settings")
	outputPath := flags.String("output", "", "new .gitsss-backup output path")
	passphraseEnv := flags.String("passphrase-env", "GITSSS_BACKUP_PASSPHRASE", "environment variable containing the backup passphrase")
	agentID := flags.String("agent-id", "", "stable agent device ID")
	version := flags.String("gitea-version", "", "detected Gitea version when known")
	maxEntries := flags.Int("max-entries", 10_000, "maximum files scanned per asset category")
	flags.Parse(args)
	if strings.TrimSpace(*agentID) == "" || strings.TrimSpace(*outputPath) == "" || strings.TrimSpace(*passphraseEnv) == "" {
		fmt.Fprintln(os.Stderr, "--agent-id, --output and --passphrase-env are required")
		os.Exit(2)
	}
	resolvedConfig := strings.TrimSpace(*configPath)
	if resolvedConfig == "" {
		paths := takeover.DiscoverConfigPaths()
		if len(paths) != 1 {
			fmt.Fprintln(os.Stderr, "auto-discovery requires exactly one Gitea app.ini; pass --config explicitly")
			os.Exit(1)
		}
		resolvedConfig = paths[0]
	}
	passphrase := os.Getenv(*passphraseEnv)
	if len(passphrase) < 12 {
		fmt.Fprintf(os.Stderr, "%s must contain a passphrase of at least 12 characters\n", *passphraseEnv)
		os.Exit(1)
	}
	report, err := takeover.Assess(takeover.Options{
		AgentID: *agentID, ConfigPath: resolvedConfig, WorkPath: *workPath,
		GiteaVersion: *version, MaxEntries: *maxEntries,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	plan, err := backup.BuildPlan(report, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := backup.Export(*outputPath, passphrase, plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var bytes int64
	for _, entry := range plan.Manifest.Entries {
		bytes += entry.Size
	}
	fmt.Printf("backup exported: %s (%d files, %d bytes, assessment %s)\n", *outputPath, len(plan.Manifest.Entries), bytes, report.ID)
}

func backupVerify(args []string) {
	flags := flag.NewFlagSet("backup-verify", flag.ExitOnError)
	inputPath := flags.String("input", "", "existing .gitsss-backup path")
	passphraseEnv := flags.String("passphrase-env", "GITSSS_BACKUP_PASSPHRASE", "environment variable containing the backup passphrase")
	flags.Parse(args)
	if strings.TrimSpace(*inputPath) == "" || strings.TrimSpace(*passphraseEnv) == "" {
		fmt.Fprintln(os.Stderr, "--input and --passphrase-env are required")
		os.Exit(2)
	}
	passphrase := os.Getenv(*passphraseEnv)
	if passphrase == "" {
		fmt.Fprintf(os.Stderr, "%s is empty\n", *passphraseEnv)
		os.Exit(1)
	}
	verification, err := backup.Verify(*inputPath, passphrase)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(verification); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func probe(args []string) {
	flags := flag.NewFlagSet("probe", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "agent token")
	flags.Parse(args)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "--server and --token are required")
		os.Exit(2)
	}

	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(*serverURL, "/")+"/api/v1/dashboard", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request.Header.Set("Authorization", "Bearer "+*token)
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		fmt.Fprintf(os.Stderr, "probe failed: %s: %s\n", response.Status, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var snapshot struct {
		Metrics []struct {
			Label string `json:"label"`
			Value int    `json:"value"`
		} `json:"metrics"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("agent connected: %d dashboard metrics available\n", len(snapshot.Metrics))
}

func runnerReport(args []string) {
	flags := flag.NewFlagSet("runner-report", flag.ExitOnError)
	serverURL := flags.String("server", "", "gitsss API base URL")
	token := flags.String("token", "", "agent token")
	id := flags.String("id", "", "stable agent id")
	platform := flags.String("platform", "", "runner platform, for example windows/amd64")
	version := flags.String("version", "", "installed runner version")
	flags.Parse(args)
	if strings.TrimSpace(*serverURL) == "" || strings.TrimSpace(*token) == "" || strings.TrimSpace(*id) == "" || strings.TrimSpace(*platform) == "" {
		fmt.Fprintln(os.Stderr, "--server, --token, --id and --platform are required")
		os.Exit(2)
	}

	payload, err := json.Marshal(runner.Report{ID: *id, Platform: *platform, Version: *version})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(*serverURL, "/")+"/api/v1/runners/reconcile", bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	request.Header.Set("Authorization", "Bearer "+*token)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		fmt.Fprintf(os.Stderr, "runner report failed: %s: %s\n", response.Status, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	var reconciliation runner.Reconciliation
	if err := json.NewDecoder(response.Body).Decode(&reconciliation); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("runner action: %s (%s)\n", reconciliation.Action, reconciliation.Message)
}
