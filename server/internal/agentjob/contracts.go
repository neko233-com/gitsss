// Copyright 2026 gitsss Authors. All rights reserved.

package agentjob

import "time"

type BackupExportInput struct {
	AgentID              string `json:"agent_id"`
	ConfigPath           string `json:"config_path"`
	WorkPath             string `json:"work_path,omitempty"`
	GiteaVersion         string `json:"gitea_version,omitempty"`
	MaxEntries           int    `json:"max_entries,omitempty"`
	OutputPath           string `json:"output_path"`
	PassphraseEnv        string `json:"passphrase_env"`
	ExpectedAssessmentID string `json:"expected_assessment_id"`
}

type BackupExportResult struct {
	AssessmentID    string `json:"assessment_id"`
	OutputPath      string `json:"output_path"`
	Files           int    `json:"files"`
	SourceBytes     int64  `json:"source_bytes"`
	ContainerBytes  int64  `json:"container_bytes"`
	ContainerSHA256 string `json:"container_sha256"`
}

type BackupRestoreInput struct {
	AgentID                 string `json:"agent_id"`
	InputPath               string `json:"input_path"`
	PassphraseEnv           string `json:"passphrase_env"`
	TargetPath              string `json:"target_path"`
	ExpectedAssessmentID    string `json:"expected_assessment_id"`
	ExpectedContainerSHA256 string `json:"expected_container_sha256"`
}

type BackupRestoreResult struct {
	AssessmentID    string    `json:"assessment_id"`
	Target          string    `json:"target"`
	ContentPath     string    `json:"content_path"`
	Files           int       `json:"files"`
	Bytes           int64     `json:"bytes"`
	ContainerSHA256 string    `json:"container_sha256"`
	RestoredAt      time.Time `json:"restored_at"`
}

type BackupAuditInput struct {
	AgentID                 string `json:"agent_id"`
	InputPath               string `json:"input_path"`
	PassphraseEnv           string `json:"passphrase_env"`
	TargetPath              string `json:"target_path"`
	ExpectedAssessmentID    string `json:"expected_assessment_id"`
	ExpectedContainerSHA256 string `json:"expected_container_sha256"`
}

type BackupAuditFileSummary struct {
	New       int `json:"new"`
	Identical int `json:"identical"`
	Conflicts int `json:"conflicts"`
	Blocked   int `json:"blocked"`
}

type BackupAuditGit struct {
	Path            string `json:"path"`
	Valid           bool   `json:"valid"`
	Objects         int64  `json:"objects"`
	Refs            int64  `json:"refs"`
	ReferenceSHA256 string `json:"reference_sha256,omitempty"`
	Error           string `json:"error,omitempty"`
}

type BackupAuditLFSIssue struct {
	Path     string `json:"path"`
	Expected string `json:"expected_oid"`
	Actual   string `json:"actual_sha256"`
}

type BackupAuditResult struct {
	AssessmentID       string                 `json:"assessment_id"`
	Target             string                 `json:"target"`
	Files              BackupAuditFileSummary `json:"files"`
	Git                []BackupAuditGit       `json:"git"`
	LFSObjects         int                    `json:"lfs_objects"`
	LFSInvalid         []BackupAuditLFSIssue  `json:"lfs_invalid,omitempty"`
	Findings           []string               `json:"findings,omitempty"`
	ReadyForShadowMode bool                   `json:"ready_for_shadow_mode"`
	ContainerSHA256    string                 `json:"container_sha256"`
	AuditedAt          time.Time              `json:"audited_at"`
}
