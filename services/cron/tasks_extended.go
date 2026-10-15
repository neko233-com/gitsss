// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"strings"
	"time"

	activities_model "github.com/neko233-com/gitsss/models/activities"
	"github.com/neko233-com/gitsss/models/system"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/git/gitcmd"
	issue_indexer "github.com/neko233-com/gitsss/modules/indexer/issues"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/updatechecker"
	asymkey_service "github.com/neko233-com/gitsss/services/asymkey"
	repo_service "github.com/neko233-com/gitsss/services/repository"
	archiver_service "github.com/neko233-com/gitsss/services/repository/archiver"
	user_service "github.com/neko233-com/gitsss/services/user"
)

func registerDeleteInactiveUsers() {
	RegisterTaskFatal("delete_inactive_accounts", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@annually",
		},
		OlderThan: time.Minute * time.Duration(setting.Service.ActiveCodeLives),
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return user_service.DeleteInactiveUsers(ctx, olderThanConfig.OlderThan)
	})
}

func registerDeleteRepositoryArchives() {
	RegisterTaskFatal("delete_repo_archives", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return archiver_service.DeleteRepositoryArchives(ctx)
	})
}

func registerGarbageCollectRepositories() {
	type RepoHealthCheckConfig struct {
		BaseConfig
		Timeout time.Duration
		Args    []string `delim:" "`
	}
	RegisterTaskFatal("git_gc_repos", &RepoHealthCheckConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 72h",
		},
		Timeout: time.Duration(setting.Git.Timeout.GC) * time.Second,
		Args:    setting.Git.GCArgs,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		rhcConfig := config.(*RepoHealthCheckConfig)
		// the git args are set by config, they can be safe to be trusted
		return repo_service.GitGcRepos(ctx, rhcConfig.Timeout, gitcmd.ToTrustedCmdArgs(rhcConfig.Args))
	})
}

func registerRewriteAllPublicKeys() {
	RegisterTaskFatal("resync_all_sshkeys", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return asymkey_service.RewriteAllPublicKeys(ctx)
	})
}

func registerRewriteAllPrincipalKeys() {
	RegisterTaskFatal("resync_all_sshprincipals", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return asymkey_service.RewriteAllPrincipalKeys(ctx)
	})
}

func registerRepositoryUpdateHook() {
	RegisterTaskFatal("resync_all_hooks", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.SyncRepositoryHooks(ctx)
	})
}

func registerReinitMissingRepositories() {
	RegisterTaskFatal("reinit_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.ReinitMissingRepositories(ctx)
	})
}

func registerDeleteMissingRepositories() {
	RegisterTaskFatal("delete_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, user *user_model.User, _ Config) error {
		return repo_service.DeleteMissingRepositories(ctx, user)
	})
}

func registerRemoveRandomAvatars() {
	RegisterTaskFatal("delete_generated_repository_avatars", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.RemoveRandomAvatars(ctx)
	})
}

func registerDeleteOldActions() {
	RegisterTaskFatal("delete_old_actions", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		OlderThan: 365 * 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return activities_model.DeleteOldActions(ctx, olderThanConfig.OlderThan)
	})
}

// UpdateCheckerConfig is the [cron.update_checker] task.
// CHECK_GITHUB defaults to false so an enabled task with an empty endpoint still makes no network request.
type UpdateCheckerConfig struct {
	BaseConfig
	HTTPEndpoint string        `ini:"HTTP_ENDPOINT"`
	CheckGitHub  bool          `ini:"CHECK_GITHUB"`
	GitHubRepo   string        `ini:"GITHUB_REPO"`
	Mirrors      string        `ini:"MIRRORS"`
	ProbeTimeout time.Duration `ini:"PROBE_TIMEOUT"`
}

func (c *UpdateCheckerConfig) options() updatechecker.Options {
	mirrors, _ := updatechecker.ParseMirrors(c.Mirrors)
	return updatechecker.Options{
		HTTPEndpoint: strings.TrimSpace(c.HTTPEndpoint),
		CheckGitHub:  c.CheckGitHub,
		GitHubRepo:   strings.TrimSpace(c.GitHubRepo),
		Mirrors:      mirrors,
		ProbeTimeout: c.ProbeTimeout,
	}
}

func registerUpdateGitSSSChecker() {
	RegisterTaskFatal("update_checker", &UpdateCheckerConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		opts := config.(*UpdateCheckerConfig).options()
		// Prefer the live config so mirror changes from the admin page apply before the next process start.
		if setting.CfgProvider != nil {
			opts = updatechecker.OptionsFromConfig()
		}
		opts.CurrentVersion = setting.AppVer
		if !opts.CheckGitHub && strings.TrimSpace(opts.HTTPEndpoint) == "" {
			return nil
		}
		report, err := updatechecker.Check(ctx, opts)
		if err != nil {
			return err
		}
		if strings.TrimSpace(report.LatestVersion) == "" {
			return nil
		}
		if err := updatechecker.SaveReport(ctx, report); err != nil {
			return err
		}
		return updatechecker.UpdateRemoteVersion(ctx, report.LatestVersion)
	})
}

func registerDeleteOldSystemNotices() {
	RegisterTaskFatal("delete_old_system_notices", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		OlderThan: 365 * 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return system.DeleteOldSystemNotices(ctx, olderThanConfig.OlderThan)
	})
}

type GCLFSConfig struct {
	BaseConfig
	OlderThan                time.Duration
	LastUpdatedMoreThanAgo   time.Duration
	NumberToCheckPerRepo     int64
	ProportionToCheckPerRepo float64
}

func registerGCLFS() {
	if !setting.LFS.StartServer {
		return
	}

	RegisterTaskFatal("gc_lfs", &GCLFSConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 24h",
		},
		// Only attempt to garbage collect lfs meta objects older than a week as the order of git lfs upload
		// and git object upload is not necessarily guaranteed. It's possible to imagine a situation whereby
		// an LFS object is uploaded but the git branch is not uploaded immediately, or there are some rapid
		// changes in new branches that might lead to lfs objects becoming temporarily unassociated with git
		// objects.
		//
		// It is likely that a week is potentially excessive but it should definitely be enough that any
		// unassociated LFS object is genuinely unassociated.
		OlderThan: 24 * time.Hour * 7,

		// Only GC things that haven't been looked at in the past 3 days
		LastUpdatedMoreThanAgo:   24 * time.Hour * 3,
		NumberToCheckPerRepo:     100,
		ProportionToCheckPerRepo: 0.6,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		gcLFSConfig := config.(*GCLFSConfig)
		return repo_service.GarbageCollectLFSMetaObjects(ctx, repo_service.GarbageCollectLFSMetaObjectsOptions{
			AutoFix:                 true,
			OlderThan:               time.Now().Add(-gcLFSConfig.OlderThan),
			UpdatedLessRecentlyThan: time.Now().Add(-gcLFSConfig.LastUpdatedMoreThanAgo),
		})
	})
}

func registerRebuildIssueIndexer() {
	RegisterTaskFatal("rebuild_issue_indexer", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		return issue_indexer.PopulateIssueIndexer(ctx)
	})
}

func initExtendedTasks() {
	registerDeleteInactiveUsers()
	registerDeleteRepositoryArchives()
	registerGarbageCollectRepositories()
	registerRewriteAllPublicKeys()
	registerRewriteAllPrincipalKeys()
	registerRepositoryUpdateHook()
	registerReinitMissingRepositories()
	registerDeleteMissingRepositories()
	registerRemoveRandomAvatars()
	registerDeleteOldActions()
	registerUpdateGitSSSChecker()
	registerDeleteOldSystemNotices()
	registerGCLFS()
	registerRebuildIssueIndexer()
}
