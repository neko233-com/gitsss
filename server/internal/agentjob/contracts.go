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
