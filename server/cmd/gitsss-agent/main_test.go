// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/backup"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestExecuteAgentJobExportsConfirmedEncryptedBackup(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mssql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := takeover.Assess(takeover.Options{AgentID: "old-server", ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_AGENT_JOB_PASSPHRASE", "Agent job test passphrase")
	outputPath := filepath.Join(directory, "confirmed.gitsss-backup")
	input, err := json.Marshal(agentjob.BackupExportInput{
		AgentID: "old-server", ConfigPath: configPath, OutputPath: outputPath,
		PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE", ExpectedAssessmentID: report.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupExport, Input: input}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var result agentjob.BackupExportResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	container, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(container)
	if result.AssessmentID != report.ID || result.OutputPath != outputPath || result.Files != 1 || result.SourceBytes == 0 || result.ContainerBytes != int64(len(container)) || result.ContainerSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("unexpected backup result: %#v", result)
	}
}

func TestExecuteAgentJobRejectsStaleBackupConfirmation(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mssql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := takeover.Assess(takeover.Options{AgentID: "old-server", ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = postgres\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_AGENT_JOB_PASSPHRASE", "Agent job test passphrase")
	outputPath := filepath.Join(directory, "rejected.gitsss-backup")
	input, err := json.Marshal(agentjob.BackupExportInput{
		AgentID: "old-server", ConfigPath: configPath, OutputPath: outputPath,
		PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE", ExpectedAssessmentID: report.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupExport, Input: input}, "old-server"); err == nil {
		t.Fatal("stale confirmation was accepted")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("rejected backup created output: %v", err)
	}
}

func TestExecuteAgentJobChecksFreezeState(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[server]\nHTTP_ADDR = 127.0.0.1\nHTTP_PORT = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(agentjob.FreezeCheckInput{AgentID: "old-server", ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindFreezeCheck, Input: input}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var evidence takeover.FreezeEvidence
	if err := json.Unmarshal(encoded, &evidence); err != nil {
		t.Fatal(err)
	}
	if !evidence.Offline || evidence.ID == "" || evidence.AgentID != "old-server" {
		t.Fatalf("freeze evidence = %#v", evidence)
	}
}

func TestExecuteAgentJobRejectsRemoteBackupWithoutFreeze(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	configuration := "[server]\nHTTP_ADDR = 127.0.0.1\nHTTP_PORT = 1\n[database]\nDB_TYPE = postgres\nHOST = 127.0.0.1:5432\nNAME = gitea\nUSER = gitsss\nPASSWD = hidden\n"
	if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := takeover.Assess(takeover.Options{AgentID: "old-server", ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_AGENT_JOB_PASSPHRASE", "Agent job test passphrase")
	input, err := json.Marshal(agentjob.BackupExportInput{
		AgentID: "old-server", ConfigPath: configPath, OutputPath: filepath.Join(directory, "rejected.gitsss-backup"),
		PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE", ExpectedAssessmentID: report.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupExport, Input: input}, "old-server"); err == nil || !strings.Contains(err.Error(), "freeze") {
		t.Fatalf("remote backup without freeze evidence = %v", err)
	}
}

func TestExecuteAgentJobRestoresConfirmedContainer(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mssql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := takeover.Assess(takeover.Options{AgentID: "old-server", ConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_AGENT_JOB_PASSPHRASE", "Agent job test passphrase")
	backupPath := filepath.Join(directory, "confirmed.gitsss-backup")
	exportInput, err := json.Marshal(agentjob.BackupExportInput{
		AgentID: "old-server", ConfigPath: configPath, OutputPath: backupPath,
		PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE", ExpectedAssessmentID: report.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	exported, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupExport, Input: exportInput}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var exportResult agentjob.BackupExportResult
	if err := json.Unmarshal(exported, &exportResult); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(directory, "isolated")
	restoreInput, err := json.Marshal(agentjob.BackupRestoreInput{
		AgentID: "old-server", InputPath: backupPath, PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE",
		TargetPath: targetPath, ExpectedAssessmentID: report.ID, ExpectedContainerSHA256: exportResult.ContainerSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupRestore, Input: restoreInput}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var restoreResult agentjob.BackupRestoreResult
	if err := json.Unmarshal(restored, &restoreResult); err != nil {
		t.Fatal(err)
	}
	if restoreResult.AssessmentID != report.ID || restoreResult.ContainerSHA256 != exportResult.ContainerSHA256 || restoreResult.Target != targetPath || restoreResult.Files != 1 {
		t.Fatalf("unexpected restore result: %#v", restoreResult)
	}
	content, err := os.ReadFile(filepath.Join(restoreResult.ContentPath, "config", "app.ini"))
	if err != nil || string(content) != "[database]\nDB_TYPE = mssql\n" {
		t.Fatalf("restored configuration = %q, %v", content, err)
	}
	auditInput, err := json.Marshal(agentjob.BackupAuditInput{
		AgentID: "old-server", InputPath: backupPath, PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE",
		TargetPath: targetPath, ExpectedAssessmentID: report.ID, ExpectedContainerSHA256: exportResult.ContainerSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	audited, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindBackupAudit, Input: auditInput}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var auditResult agentjob.BackupAuditResult
	if err := json.Unmarshal(audited, &auditResult); err != nil {
		t.Fatal(err)
	}
	if !auditResult.ReadyForShadowMode || auditResult.Files.Identical != restoreResult.Files || auditResult.ContainerSHA256 != exportResult.ContainerSHA256 || auditResult.Git == nil {
		t.Fatalf("unexpected audit result: %#v", auditResult)
	}
	shadowInput, err := json.Marshal(agentjob.ShadowAuditInput{
		AgentID: "old-server", ConfigPath: configPath, InputPath: backupPath,
		PassphraseEnv: "GITSSS_AGENT_JOB_PASSPHRASE", TargetPath: targetPath,
		ExpectedAssessmentID: report.ID, ExpectedContainerSHA256: exportResult.ContainerSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	shadowed, err := executeAgentJob(agentjob.Job{Kind: agentjob.KindShadowAudit, Input: shadowInput}, "old-server")
	if err != nil {
		t.Fatal(err)
	}
	var shadowResult backup.ShadowAudit
	if err := json.Unmarshal(shadowed, &shadowResult); err != nil {
		t.Fatal(err)
	}
	if shadowResult.ReadyForShadowMode {
		t.Fatalf("remote database without a snapshot was accepted: %#v", shadowResult)
	}
}
