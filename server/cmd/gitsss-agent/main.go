// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agenthttp"
	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
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
	case "backup-import-plan":
		backupImportPlan(os.Args[2:])
	case "backup-restore-isolated":
		backupRestoreIsolated(os.Args[2:])
	case "backup-audit-restore":
		backupAuditRestore(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	case "work-once":
		workOnce(os.Args[2:])
	case "work":
		work(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: gitsss-agent <serve|work|work-once|probe|runner-report|assess|backup-export|backup-verify|backup-import-plan|backup-restore-isolated|backup-audit-restore> [options]")
}

func workOnce(args []string) {
	runWorkOnce(args)
}

func claimAgentJob(serverURL, token, agentID string) (agentjob.Job, bool, error) {
	return claimAgentJobContext(context.Background(), &http.Client{Timeout: 30 * time.Second}, serverURL, token, agentID)
}

func claimAgentJobContext(ctx context.Context, client *http.Client, serverURL, token, agentID string) (agentjob.Job, bool, error) {
	payload, _ := json.Marshal(map[string]string{"agent_id": agentID})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(serverURL, "/")+"/api/v1/agent/jobs/claim", bytes.NewReader(payload))
	if err != nil {
		return agentjob.Job{}, false, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return agentjob.Job{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return agentjob.Job{}, false, nil
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return agentjob.Job{}, false, fmt.Errorf("claim Agent job: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var job agentjob.Job
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		return agentjob.Job{}, false, err
	}
	return job, true, nil
}

func executeAgentJob(job agentjob.Job, agentID string) (json.RawMessage, error) {
	switch job.Kind {
	case agentjob.KindAssessment:
		var options takeover.Options
		if err := json.Unmarshal(job.Input, &options); err != nil {
			return nil, err
		}
		if options.AgentID == "" {
			options.AgentID = agentID
		}
		if options.AgentID != agentID {
			return nil, errors.New("assessment Agent ID does not match the worker")
		}
		report, err := takeover.Assess(options)
		if err != nil {
			return nil, err
		}
		return json.Marshal(report)
	case agentjob.KindBackupVerify:
		var input struct {
			InputPath     string `json:"input_path"`
			PassphraseEnv string `json:"passphrase_env"`
		}
		if err := json.Unmarshal(job.Input, &input); err != nil {
			return nil, err
		}
		passphrase := os.Getenv(input.PassphraseEnv)
		if strings.TrimSpace(input.InputPath) == "" || !validAgentEnvironmentName(input.PassphraseEnv) || passphrase == "" {
			return nil, errors.New("backup input path and passphrase environment are required")
		}
		verification, err := backup.Verify(input.InputPath, passphrase)
		if err != nil {
			return nil, err
		}
		return json.Marshal(verification)
	case agentjob.KindBackupExport:
		var input agentjob.BackupExportInput
		if err := json.Unmarshal(job.Input, &input); err != nil {
			return nil, err
		}
		if input.AgentID == "" {
			input.AgentID = agentID
		}
		if input.AgentID != agentID {
			return nil, errors.New("backup Agent ID does not match the worker")
		}
		if strings.TrimSpace(input.OutputPath) == "" || !validAgentEnvironmentName(input.PassphraseEnv) || len(os.Getenv(input.PassphraseEnv)) < 12 {
			return nil, errors.New("backup output path and a valid passphrase environment are required")
		}
		report, err := takeover.Assess(takeover.Options{
			AgentID: input.AgentID, ConfigPath: input.ConfigPath, WorkPath: input.WorkPath,
			GiteaVersion: input.GiteaVersion, MaxEntries: input.MaxEntries,
		})
		if err != nil {
			return nil, err
		}
		if input.ExpectedAssessmentID == "" || report.ID != input.ExpectedAssessmentID {
			return nil, errors.New("backup confirmation does not match the current assessment")
		}
		plan, err := backup.BuildPlan(report, time.Now())
		if err != nil {
			return nil, err
		}
		if err := backup.Export(input.OutputPath, os.Getenv(input.PassphraseEnv), plan); err != nil {
			return nil, err
		}
		containerBytes, containerDigest, err := hashAgentFile(input.OutputPath)
		if err != nil {
			return nil, err
		}
		result := agentjob.BackupExportResult{
			AssessmentID: report.ID, OutputPath: input.OutputPath, Files: len(plan.Manifest.Entries),
			ContainerBytes: containerBytes, ContainerSHA256: containerDigest,
		}
		for _, entry := range plan.Manifest.Entries {
			result.SourceBytes += entry.Size
		}
		return json.Marshal(result)
	case agentjob.KindBackupRestore:
		var input agentjob.BackupRestoreInput
		if err := json.Unmarshal(job.Input, &input); err != nil {
			return nil, err
		}
		if input.AgentID == "" {
			input.AgentID = agentID
		}
		passphrase := os.Getenv(input.PassphraseEnv)
		if input.AgentID != agentID {
			return nil, errors.New("restore Agent ID does not match the worker")
		}
		if strings.TrimSpace(input.InputPath) == "" || strings.TrimSpace(input.TargetPath) == "" || strings.TrimSpace(input.ExpectedAssessmentID) == "" || !validAgentEnvironmentName(input.PassphraseEnv) || len(passphrase) < 12 || !validAgentSHA256(input.ExpectedContainerSHA256) {
			return nil, errors.New("restore paths, confirmation, digest and a valid passphrase environment are required")
		}
		restored, err := backup.RestoreIsolatedExact(input.InputPath, passphrase, input.TargetPath, input.ExpectedAssessmentID, input.ExpectedContainerSHA256)
		if err != nil {
			return nil, err
		}
		return json.Marshal(agentjob.BackupRestoreResult{
			AssessmentID: restored.AssessmentID, Target: restored.Target, ContentPath: restored.ContentPath,
			Files: restored.Files, Bytes: restored.Bytes, ContainerSHA256: input.ExpectedContainerSHA256, RestoredAt: restored.RestoredAt,
		})
	case agentjob.KindBackupAudit:
		var input agentjob.BackupAuditInput
		if err := json.Unmarshal(job.Input, &input); err != nil {
			return nil, err
		}
		if input.AgentID == "" {
			input.AgentID = agentID
		}
		passphrase := os.Getenv(input.PassphraseEnv)
		if input.AgentID != agentID {
			return nil, errors.New("audit Agent ID does not match the worker")
		}
		if strings.TrimSpace(input.InputPath) == "" || strings.TrimSpace(input.TargetPath) == "" || strings.TrimSpace(input.ExpectedAssessmentID) == "" || !validAgentEnvironmentName(input.PassphraseEnv) || len(passphrase) < 12 || !validAgentSHA256(input.ExpectedContainerSHA256) {
			return nil, errors.New("audit paths, confirmation, digest and a valid passphrase environment are required")
		}
		audit, err := backup.AuditRestoreExact(input.InputPath, passphrase, input.TargetPath, input.ExpectedAssessmentID, input.ExpectedContainerSHA256)
		if err != nil {
			return nil, err
		}
		result := agentjob.BackupAuditResult{
			AssessmentID: audit.AssessmentID, Target: audit.Target,
			Files: agentjob.BackupAuditFileSummary{
				New: audit.Files.New, Identical: audit.Files.Identical, Conflicts: audit.Files.Conflicts, Blocked: audit.Files.Blocked,
			},
			Git: make([]agentjob.BackupAuditGit, 0), LFSObjects: audit.LFS.Objects,
			Findings: audit.Findings, ReadyForShadowMode: audit.ReadyForShadowMode,
			ContainerSHA256: input.ExpectedContainerSHA256, AuditedAt: audit.AuditedAt,
		}
		for _, repository := range audit.Git {
			result.Git = append(result.Git, agentjob.BackupAuditGit{
				Path: repository.Path, Valid: repository.Valid, Objects: repository.Objects, Refs: repository.Refs,
				ReferenceSHA256: repository.ReferenceSHA256, Error: repository.Error,
			})
		}
		for _, issue := range audit.LFS.Invalid {
			result.LFSInvalid = append(result.LFSInvalid, agentjob.BackupAuditLFSIssue{
				Path: issue.Path, Expected: issue.Expected, Actual: issue.Actual,
			})
		}
		return json.Marshal(result)
	default:
		return nil, errors.New("unsupported Agent job kind")
	}
}

func validAgentEnvironmentName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func validAgentSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func hashAgentFile(filePath string) (int64, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hasher := sha256.New()
	written, err := io.Copy(hasher, file)
	if err != nil {
		return 0, "", err
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func completeAgentJob(serverURL, token, agentID, jobID string, result json.RawMessage, failure string) (agentjob.Job, error) {
	return completeAgentJobContext(context.Background(), &http.Client{Timeout: 30 * time.Second}, serverURL, token, agentID, jobID, result, failure)
}

func completeAgentJobContext(ctx context.Context, client *http.Client, serverURL, token, agentID, jobID string, result json.RawMessage, failure string) (agentjob.Job, error) {
	payload, err := json.Marshal(struct {
		AgentID string          `json:"agent_id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   string          `json:"error,omitempty"`
	}{AgentID: agentID, Result: result, Error: failure})
	if err != nil {
		return agentjob.Job{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(serverURL, "/")+"/api/v1/agent/jobs/"+jobID+"/complete", bytes.NewReader(payload))
	if err != nil {
		return agentjob.Job{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return agentjob.Job{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return agentjob.Job{}, fmt.Errorf("complete Agent job: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var job agentjob.Job
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		return agentjob.Job{}, err
	}
	return job, nil
}

func serve(args []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	listenAddress := flags.String("listen", "127.0.0.1:8790", "loopback listen address")
	tokenEnv := flags.String("token-env", "GITSSS_AGENT_HTTP_TOKEN", "environment variable containing the HTTP token")
	tokenID := flags.String("token-id", "local-agent-http", "non-secret token identifier for audit records")
	tokenTTL := flags.Duration("token-ttl", 24*time.Hour, "token lifetime; rotate by restarting with a new token")
	scopes := flags.String("scopes", "agent:read,takeover:assess,backup:verify,audit:read", "comma-separated token scopes")
	flags.Parse(args)
	if err := agenthttp.ValidateLoopbackAddress(*listenAddress); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if strings.TrimSpace(*tokenEnv) == "" || strings.TrimSpace(*tokenID) == "" || *tokenTTL <= 0 || *tokenTTL > 30*24*time.Hour {
		fmt.Fprintln(os.Stderr, "--token-env, --token-id and a --token-ttl up to 720h are required")
		os.Exit(2)
	}
	token := os.Getenv(*tokenEnv)
	if len(token) < 24 {
		fmt.Fprintf(os.Stderr, "%s must contain at least 24 characters\n", *tokenEnv)
		os.Exit(1)
	}
	allowedScopes := make([]string, 0)
	for _, scope := range strings.Split(*scopes, ",") {
		if scope = strings.TrimSpace(scope); scope != "" {
			allowedScopes = append(allowedScopes, scope)
		}
	}
	if len(allowedScopes) == 0 {
		fmt.Fprintln(os.Stderr, "--scopes must contain at least one scope")
		os.Exit(2)
	}
	expiresAt := time.Now().UTC().Add(*tokenTTL)
	verifier := auth.NewVerifier(auth.NewRecord(*tokenID, token, expiresAt, allowedScopes...))
	handler := agenthttp.NewServer(verifier, &audit.Log{}).Handler()
	server := &http.Server{
		Addr: *listenAddress, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	fmt.Printf("gitsss Agent HTTP listening on http://%s (token %s expires %s)\n", *listenAddress, *tokenID, expiresAt.Format(time.RFC3339))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

func backupImportPlan(args []string) {
	flags := flag.NewFlagSet("backup-import-plan", flag.ExitOnError)
	inputPath := flags.String("input", "", "existing .gitsss-backup path")
	targetPath := flags.String("target", "", "isolated import target to inspect without writing")
	passphraseEnv := flags.String("passphrase-env", "GITSSS_BACKUP_PASSPHRASE", "environment variable containing the backup passphrase")
	flags.Parse(args)
	if strings.TrimSpace(*inputPath) == "" || strings.TrimSpace(*targetPath) == "" || strings.TrimSpace(*passphraseEnv) == "" {
		fmt.Fprintln(os.Stderr, "--input, --target and --passphrase-env are required")
		os.Exit(2)
	}
	passphrase := os.Getenv(*passphraseEnv)
	if passphrase == "" {
		fmt.Fprintf(os.Stderr, "%s is empty\n", *passphraseEnv)
		os.Exit(1)
	}
	plan, err := backup.PreviewImport(*inputPath, passphrase, *targetPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func backupRestoreIsolated(args []string) {
	flags := flag.NewFlagSet("backup-restore-isolated", flag.ExitOnError)
	inputPath := flags.String("input", "", "existing .gitsss-backup path")
	targetPath := flags.String("target", "", "new isolated restore directory")
	confirmation := flags.String("confirm", "", "exact assessment ID shown by backup-verify")
	passphraseEnv := flags.String("passphrase-env", "GITSSS_BACKUP_PASSPHRASE", "environment variable containing the backup passphrase")
	flags.Parse(args)
	if strings.TrimSpace(*inputPath) == "" || strings.TrimSpace(*targetPath) == "" || strings.TrimSpace(*confirmation) == "" || strings.TrimSpace(*passphraseEnv) == "" {
		fmt.Fprintln(os.Stderr, "--input, --target, --confirm and --passphrase-env are required")
		os.Exit(2)
	}
	passphrase := os.Getenv(*passphraseEnv)
	if passphrase == "" {
		fmt.Fprintf(os.Stderr, "%s is empty\n", *passphraseEnv)
		os.Exit(1)
	}
	result, err := backup.RestoreIsolated(*inputPath, passphrase, *targetPath, *confirmation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func backupAuditRestore(args []string) {
	flags := flag.NewFlagSet("backup-audit-restore", flag.ExitOnError)
	inputPath := flags.String("input", "", "existing .gitsss-backup path")
	targetPath := flags.String("target", "", "isolated restore directory containing content")
	passphraseEnv := flags.String("passphrase-env", "GITSSS_BACKUP_PASSPHRASE", "environment variable containing the backup passphrase")
	flags.Parse(args)
	if strings.TrimSpace(*inputPath) == "" || strings.TrimSpace(*targetPath) == "" || strings.TrimSpace(*passphraseEnv) == "" {
		fmt.Fprintln(os.Stderr, "--input, --target and --passphrase-env are required")
		os.Exit(2)
	}
	passphrase := os.Getenv(*passphraseEnv)
	if passphrase == "" {
		fmt.Fprintf(os.Stderr, "%s is empty\n", *passphraseEnv)
		os.Exit(1)
	}
	audit, err := backup.AuditRestore(*inputPath, passphrase, *targetPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(audit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !audit.ReadyForShadowMode {
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
