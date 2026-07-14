// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestExecuteAgentJobExportsConfirmedEncryptedBackup(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mysql\n"), 0o600); err != nil {
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
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mysql\n"), 0o600); err != nil {
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

func TestExecuteAgentJobRestoresConfirmedContainer(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = mysql\n"), 0o600); err != nil {
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
	if err != nil || string(content) != "[database]\nDB_TYPE = mysql\n" {
		t.Fatalf("restored configuration = %q, %v", content, err)
	}
}
