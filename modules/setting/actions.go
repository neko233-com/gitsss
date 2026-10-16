// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/log"
)

const defaultMaxRerunAttempts = 50

const defaultMaxConcurrentTaskPicks = 16

// Actions settings
var (
	Actions = struct {
		Enabled               bool
		BuiltinRunnerEnabled  bool              `ini:"BUILTIN_RUNNER_ENABLED"`
		BuiltinRunnerCapacity int               `ini:"BUILTIN_RUNNER_CAPACITY"`
		BuiltinRunnerLabels   []string          `ini:"BUILTIN_RUNNER_LABELS"`
		LogStorage            *Storage          // how the created logs should be stored
		LogRetentionDays      int64             `ini:"LOG_RETENTION_DAYS"`
		LogCompression        logCompression    `ini:"LOG_COMPRESSION"`
		ArtifactStorage       *Storage          // how the created artifacts should be stored
		ArtifactRetentionDays int64             `ini:"ARTIFACT_RETENTION_DAYS"`
		DefaultActionsURL     defaultActionsURL `ini:"DEFAULT_ACTIONS_URL"`
		ZombieTaskTimeout     time.Duration     `ini:"ZOMBIE_TASK_TIMEOUT"`
		EndlessTaskTimeout    time.Duration     `ini:"ENDLESS_TASK_TIMEOUT"`
		AbandonedJobTimeout   time.Duration     `ini:"ABANDONED_JOB_TIMEOUT"`
		SkipWorkflowStrings   []string          `ini:"SKIP_WORKFLOW_STRINGS"`
		WorkflowDirs          []string          `ini:"WORKFLOW_DIRS"`
		ScopedWorkflowDirs    []string          `ini:"SCOPED_WORKFLOW_DIRS"`
		MaxRerunAttempts      int64             `ini:"MAX_RERUN_ATTEMPTS"`
		// MaxConcurrentTaskPicks bounds how many runners may run the task-assignment
		// transaction at once per Gitea instance, to avoid a thundering herd when many
		// runners poll together. It is a per-process limit, not a cluster-wide one.
		MaxConcurrentTaskPicks int `ini:"MAX_CONCURRENT_TASK_PICKS"`
	}{
		Enabled:                true,
		BuiltinRunnerEnabled:   true,
		BuiltinRunnerCapacity:  2,
		BuiltinRunnerLabels:    defaultBuiltinRunnerLabels(runtime.GOOS),
		DefaultActionsURL:      defaultActionsURLSelf,
		SkipWorkflowStrings:    []string{"[skip ci]", "[ci skip]", "[no ci]", "[skip actions]", "[actions skip]"},
		WorkflowDirs:           []string{".gitea/workflows", ".github/workflows"},
		ScopedWorkflowDirs:     []string{".gitea/scoped_workflows"},
		MaxRerunAttempts:       defaultMaxRerunAttempts,
		MaxConcurrentTaskPicks: defaultMaxConcurrentTaskPicks,
	}
)

func defaultBuiltinRunnerLabels(goos string) []string {
	labels := []string{"self-hosted:host"}
	switch goos {
	case "darwin":
		return append(labels, "macos-latest:host")
	case "windows":
		return append(labels, "windows-latest:host")
	case "linux":
		return append(labels, "ubuntu-latest:host", "ubuntu-24.04:host")
	default:
		return labels
	}
}

type defaultActionsURL string

func (url defaultActionsURL) URL() string {
	switch url {
	case defaultActionsURLGitHub:
		return "https://github.com"
	case defaultActionsURLSelf:
		return strings.TrimSuffix(AppURL, "/")
	default:
		// Fail closed to the local instance so an invalid setting cannot access the internet.
		return strings.TrimSuffix(AppURL, "/")
	}
}

const (
	defaultActionsURLGitHub = "github" // https://github.com
	defaultActionsURLSelf   = "self"   // the root URL of the self-hosted Gitea instance
	// DefaultActionsURL only supports GitHub and the self-hosted Gitea.
	// It's intentionally not supported more, so please be cautious before adding more like "gitea" or "gitlab".
	// If you get some trouble with `uses: username/action_name@version` in your workflow,
	// please consider to use `uses: https://the_url_you_want_to_use/username/action_name@version` instead.
)

type logCompression string

func (c logCompression) IsValid() bool {
	return c.IsNone() || c.IsZstd()
}

func (c logCompression) IsNone() bool {
	return string(c) == "none"
}

func (c logCompression) IsZstd() bool {
	return c == "" || string(c) == "zstd"
}

func loadActionsFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("actions")
	err := sec.MapTo(&Actions)
	if err != nil {
		return fmt.Errorf("failed to map Actions settings: %v", err)
	}
	Actions.BuiltinRunnerEnabled = sec.Key("BUILTIN_RUNNER_ENABLED").MustBool(true)
	Actions.BuiltinRunnerCapacity = sec.Key("BUILTIN_RUNNER_CAPACITY").MustInt(2)
	if Actions.BuiltinRunnerCapacity <= 0 {
		Actions.BuiltinRunnerCapacity = 2
	}
	Actions.BuiltinRunnerLabels = sec.Key("BUILTIN_RUNNER_LABELS").Strings(",")
	if len(Actions.BuiltinRunnerLabels) == 0 {
		Actions.BuiltinRunnerLabels = defaultBuiltinRunnerLabels(runtime.GOOS)
	}
	for i, label := range Actions.BuiltinRunnerLabels {
		Actions.BuiltinRunnerLabels[i] = strings.TrimSpace(label)
	}
	Actions.BuiltinRunnerLabels = slices.DeleteFunc(Actions.BuiltinRunnerLabels, func(label string) bool { return label == "" })
	if len(Actions.BuiltinRunnerLabels) == 0 {
		return errors.New("[actions] BUILTIN_RUNNER_LABELS must contain at least one entry")
	}

	if urls := string(Actions.DefaultActionsURL); urls != defaultActionsURLGitHub && urls != defaultActionsURLSelf {
		url, _, _ := strings.Cut(urls, ",")
		if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
			log.Error("[actions] DEFAULT_ACTIONS_URL does not support %q as custom URL any longer, fallback to %q",
				urls,
				defaultActionsURLSelf,
			)
			Actions.DefaultActionsURL = defaultActionsURLSelf
		} else {
			return fmt.Errorf("unsupported [actions] DEFAULT_ACTIONS_URL: %q", urls)
		}
	}

	// don't support to read configuration from [actions]
	Actions.LogStorage, err = getStorage(rootCfg, "actions_log", "", nil)
	if err != nil {
		return err
	}
	// default to 1 year
	if Actions.LogRetentionDays <= 0 {
		Actions.LogRetentionDays = 365
	}

	actionsSec, _ := rootCfg.GetSection("actions.artifacts")

	Actions.ArtifactStorage, err = getStorage(rootCfg, "actions_artifacts", "", actionsSec)
	if err != nil {
		return err
	}

	// default to 90 days in Github Actions
	if Actions.ArtifactRetentionDays <= 0 {
		Actions.ArtifactRetentionDays = 90
	}

	Actions.ZombieTaskTimeout = sec.Key("ZOMBIE_TASK_TIMEOUT").MustDuration(10 * time.Minute)
	Actions.EndlessTaskTimeout = sec.Key("ENDLESS_TASK_TIMEOUT").MustDuration(3 * time.Hour)
	Actions.AbandonedJobTimeout = sec.Key("ABANDONED_JOB_TIMEOUT").MustDuration(24 * time.Hour)

	if Actions.MaxRerunAttempts <= 0 {
		Actions.MaxRerunAttempts = defaultMaxRerunAttempts
	}

	if Actions.MaxConcurrentTaskPicks <= 0 {
		Actions.MaxConcurrentTaskPicks = defaultMaxConcurrentTaskPicks
	}

	if !Actions.LogCompression.IsValid() {
		return fmt.Errorf("invalid [actions] LOG_COMPRESSION: %q", Actions.LogCompression)
	}

	workflowDirs := normalizeWorkflowDirs(Actions.WorkflowDirs)
	if len(workflowDirs) == 0 {
		return errors.New("[actions] WORKFLOW_DIRS must contain at least one entry")
	}
	Actions.WorkflowDirs = workflowDirs

	// SCOPED_WORKFLOW_DIRS may be empty (feature disabled), but it must not overlap with WORKFLOW_DIRS:
	// a scoped dir nested in (or equal to) a workflow dir would make the same file run both repo-level and scope-level.
	Actions.ScopedWorkflowDirs = normalizeWorkflowDirs(Actions.ScopedWorkflowDirs)
	for _, scopedDir := range Actions.ScopedWorkflowDirs {
		for _, workflowDir := range Actions.WorkflowDirs {
			if scopedDir == workflowDir ||
				strings.HasPrefix(scopedDir, workflowDir+"/") ||
				strings.HasPrefix(workflowDir, scopedDir+"/") {
				return fmt.Errorf("[actions] SCOPED_WORKFLOW_DIRS entry %q overlaps with WORKFLOW_DIRS entry %q", scopedDir, workflowDir)
			}
		}
	}

	return nil
}

// normalizeWorkflowDirs trims, normalizes separators and drops empty/trailing-slash entries.
func normalizeWorkflowDirs(dirs []string) []string {
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		dir = strings.ReplaceAll(dir, `\`, `/`)
		dir = strings.TrimRight(dir, "/")
		normalized = append(normalized, dir)
	}
	return normalized
}
