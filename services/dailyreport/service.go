// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package dailyreport integrates daily report rules with GitSSS activity records.
package dailyreport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	activities_model "github.com/neko233-com/gitsss/models/activities"
	"github.com/neko233-com/gitsss/models/db"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/gitrepo"
	"github.com/neko233-com/gitsss/modules/globallock"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/repository"
	"github.com/neko233-com/gitsss/modules/setting"
)

const lockKey = "daily_report_execution"

var objectIDPattern = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

func comparisonIDs(compare string) (string, string, bool) {
	_, value, ok := strings.Cut(compare, "/compare/")
	if !ok {
		return "", "", false
	}
	oldID, newID, ok := strings.Cut(value, "...")
	if !ok || !objectIDPattern.MatchString(oldID) || !objectIDPattern.MatchString(newID) || len(oldID) != len(newID) {
		return "", "", false
	}
	return oldID, newID, true
}

func completePush(ctx context.Context, repoID int64, payload *repository.PushCommits) error {
	if payload.Len == len(payload.Commits) {
		return nil
	}
	if payload.Len < 0 || payload.Len > 10000 {
		return errors.New("invalid push size")
	}
	oldID, newID, ok := comparisonIDs(payload.CompareURL)
	if !ok {
		return errors.New("push activity contains incomplete commits")
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return errors.New("cannot read push repository")
	}
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return errors.New("cannot open push repository")
	}
	defer gitRepo.Close()
	commits, err := gitRepo.CommitsBetween(git.RefName(newID), git.RefName(oldID), payload.Len+1)
	if err != nil || len(commits) != payload.Len {
		return errors.New("cannot reconstruct complete push")
	}
	payload.Commits = repository.GitToPushCommits(commits).Commits
	return nil
}

// ConfigPath keeps private provider and recipient configuration in application data.
func ConfigPath() string { return filepath.Join(setting.AppDataPath, "daily-report", "config.json") }

// LoadConfig returns a disabled default until an administrator configures the service.
func LoadConfig() (dailyreport.Config, error) {
	cfg, err := dailyreport.LoadConfig(ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, Accounts: make(map[string]string)}, nil
	}
	return cfg, err
}

// SaveConfig validates and persists the private configuration.
func SaveConfig(cfg dailyreport.Config) error { return dailyreport.SaveConfig(ConfigPath(), cfg) }

// CollectPushes reads the actor's canonical action, avoiding watcher copies.
// Oversized or incomplete action payloads fail closed instead of omitting commits.
func CollectPushes(ctx context.Context, start, end time.Time) ([]dailyreport.Push, error) {
	var actions []activities_model.Action
	err := db.GetEngine(ctx).Where("op_type = ? AND user_id = act_user_id AND created_unix >= ? AND created_unix < ?", activities_model.ActionCommitRepo, start.Unix(), end.Unix()).Limit(10001).Find(&actions)
	if err != nil {
		return nil, errors.New("cannot read push activity")
	}
	if len(actions) > 10000 {
		return nil, errors.New("too many push activities for one report")
	}
	pushes := make([]dailyreport.Push, 0, len(actions))
	for _, action := range actions {
		if action.Content == "" {
			continue
		}
		var payload repository.PushCommits
		if err := json.Unmarshal([]byte(action.Content), &payload); err != nil || action.Content == "{}" {
			return nil, errors.New("push activity contains incomplete commits")
		}
		if err := completePush(ctx, action.RepoID, &payload); err != nil {
			return nil, err
		}
		push := dailyreport.Push{RepositoryID: action.RepoID, Total: payload.Len}
		for _, commit := range payload.Commits {
			if commit == nil {
				return nil, errors.New("push activity contains invalid commit")
			}
			push.Commits = append(push.Commits, dailyreport.Commit{ID: commit.Sha1, AuthorEmail: commit.AuthorEmail, Message: commit.Message})
		}
		pushes = append(pushes, push)
	}
	return pushes, nil
}

func reportTitle(account, fullName string, date time.Time) string {
	name := account
	if fullName != "" {
		name = fullName
	}
	weekdays := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return fmt.Sprintf("%s · %s %s", name, date.Format("2006-01-02"), weekdays[date.Weekday()])
}

func filterRoleAccounts(grouped map[string][]dailyreport.AttributedCommit, cfg dailyreport.Config, resolve func(string) ([]string, error)) error {
	if !cfg.RoleFilterEnabled {
		return nil
	}
	for account := range grouped {
		labels, err := resolve(account)
		if err != nil {
			return errors.New("cannot read report role labels")
		}
		if !dailyreport.MatchesRoleLabels(true, cfg.RoleLabels, labels) {
			delete(grouped, account)
		}
	}
	return nil
}

// Execute summarizes today's pushed commits and sends cards to the configured bot.
// Both scheduled and manual runs share the same delivery receipts and lock.
func Execute(ctx context.Context, now time.Time, automatic bool) (dailyreport.RunResult, error) {
	var result dailyreport.RunResult
	locked, release, err := globallock.TryLock(ctx, lockKey)
	if err != nil {
		return result, errors.New("cannot lock daily report")
	}
	if !locked {
		return result, errors.New("a daily report is already running")
	}
	defer release()
	cfg, err := LoadConfig()
	if err != nil {
		return result, err
	}
	if automatic {
		due, err := cfg.Schedule.Due(now)
		if err != nil || !due {
			return result, err
		}
	} else if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKey == "" || cfg.Webhook == "" {
		return result, errors.New("daily report provider and webhook are not configured")
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return result, errors.New("cannot load report timezone")
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	pushes, err := CollectPushes(ctx, start, now.Add(time.Second))
	if err != nil {
		return result, err
	}
	grouped, err := dailyreport.GroupCommits(pushes, cfg.Accounts, func(email string) (string, error) {
		user, err := user_model.GetUserByEmail(ctx, email)
		if user_model.IsErrUserNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return user.Name, nil
	})
	if err != nil {
		return result, err
	}
	if err := filterRoleAccounts(grouped, cfg, func(account string) ([]string, error) {
		user, err := user_model.GetUserByName(ctx, account)
		if err != nil {
			return nil, err
		}
		return user_model.GetUserSettingJSON(ctx, user.ID, user_model.SettingsKeyRoleLabels, []string{})
	}); err != nil {
		return result, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	provider := dailyreport.ResponsesClient{HTTP: client, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model, OmitStore: cfg.OmitStore}
	store := dailyreport.FileDeliveryStore{Path: filepath.Join(setting.AppDataPath, "daily-report", "deliveries.json")}
	date := local.Format("2006-01-02")
	return dailyreport.Run(ctx, date, grouped, store, provider.Summarize, func(ctx context.Context, account, summary string) error {
		user, err := user_model.GetUserByName(ctx, account)
		if err != nil {
			return errors.New("cannot read report account")
		}
		return dailyreport.SendCard(ctx, client, cfg.Webhook, reportTitle(account, user.FullName, local), summary)
	})
}
