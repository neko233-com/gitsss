// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/neko233-com/gitsss/models/db"
	org_model "github.com/neko233-com/gitsss/models/organization"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/globallock"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
)

func orgPath(orgID int64, name string) string {
	return filepath.Join(setting.AppDataPath, "daily-report", "organizations", strconv.FormatInt(orgID, 10), name)
}

// LoadOrgConfig reads only the selected organization's private settings.
func LoadOrgConfig(orgID int64) (dailyreport.Config, error) {
	if orgID <= 0 {
		return dailyreport.Config{}, errors.New("invalid organization")
	}
	cfg, err := dailyreport.LoadConfig(orgPath(orgID, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, Accounts: make(map[string]string)}, nil
	}
	return cfg, err
}

func SaveOrgConfig(orgID int64, cfg dailyreport.Config) error {
	if orgID <= 0 {
		return errors.New("invalid organization")
	}
	return dailyreport.SaveConfig(orgPath(orgID, "config.json"), cfg)
}

// OrgMembers returns only current individual members, including private memberships.
func OrgMembers(ctx context.Context, orgID int64) ([]*user_model.User, error) {
	var members []*user_model.User
	err := db.GetEngine(ctx).Where("type = ? AND id IN (SELECT uid FROM org_user WHERE org_id = ?)", user_model.UserTypeIndividual, orgID).Asc("lower_name").Find(&members)
	return members, err
}

// ManualSelection matches explicit members or any selected role; empty is rejected.
type ManualSelection struct {
	AccountIDs []int64
	RoleLabels []string
}
type Analysis struct{ Account, FullName, Summary string }

func groupOrganization(ctx context.Context, orgID int64, cfg dailyreport.Config, now time.Time, selection *ManualSelection) (map[string][]dailyreport.AttributedCommit, error) {
	if orgID <= 0 {
		return nil, errors.New("invalid organization")
	}
	members, err := OrgMembers(ctx, orgID)
	if err != nil {
		return nil, errors.New("cannot read organization members")
	}
	kinds := make(map[string]string, len(members))
	memberIDs := make(map[int64]bool, len(members))
	for _, member := range members {
		memberIDs[member.ID] = true
	}
	selected := make(map[int64]bool)
	if selection != nil {
		if len(selection.AccountIDs) == 0 && len(selection.RoleLabels) == 0 {
			return nil, errors.New("select at least one member or role")
		}
		for _, id := range selection.AccountIDs {
			if !memberIDs[id] {
				return nil, errors.New("selected account is not an organization member")
			}
			selected[id] = true
		}
	}
	for _, member := range members {
		if selection == nil {
			kinds[member.Name] = cfg.Accounts[member.Name]
			continue
		}
		match := selected[member.ID]
		if len(selection.RoleLabels) > 0 {
			labels, err := user_model.GetUserSettingJSON(ctx, member.ID, user_model.SettingsKeyRoleLabels, []string{})
			if err != nil {
				return nil, errors.New("cannot read member role labels")
			}
			match = match || dailyreport.MatchesRoleLabels(true, selection.RoleLabels, labels)
		}
		if match {
			kinds[member.Name] = "person"
		}
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, errors.New("cannot load report timezone")
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	pushes, err := collectPushes(ctx, orgID, start, now.Add(time.Second))
	if err != nil {
		return nil, err
	}
	grouped, err := dailyreport.GroupCommits(pushes, kinds, func(email string) (string, error) {
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
		return nil, err
	}
	if selection == nil {
		err = filterRoleAccounts(grouped, cfg, func(account string) ([]string, error) {
			user, err := user_model.GetUserByName(ctx, account)
			if err != nil {
				return nil, err
			}
			return user_model.GetUserSettingJSON(ctx, user.ID, user_model.SettingsKeyRoleLabels, []string{})
		})
	}
	return grouped, err
}

// AnalyzeOrganization returns previews without sending or updating delivery receipts.
func AnalyzeOrganization(ctx context.Context, orgID int64, now time.Time, selection ManualSelection) ([]Analysis, error) {
	if len(selection.AccountIDs) == 0 && len(selection.RoleLabels) == 0 {
		return nil, errors.New("select at least one member or role")
	}
	cfg, err := LoadOrgConfig(orgID)
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKey == "" {
		return nil, errors.New("report provider is not configured")
	}
	grouped, err := groupOrganization(ctx, orgID, cfg, now, &selection)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)
	client := dailyreport.ResponsesClient{HTTP: &http.Client{Timeout: 60 * time.Second}, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model, OmitStore: cfg.OmitStore}
	result := make([]Analysis, 0, len(names))
	for _, name := range names {
		input, err := json.Marshal(grouped[name])
		if err != nil {
			return nil, errors.New("cannot prepare report input")
		}
		summary, err := client.Summarize(ctx, string(input))
		if err != nil {
			return nil, err
		}
		user, err := user_model.GetUserByName(ctx, name)
		if err != nil {
			return nil, errors.New("cannot read report account")
		}
		result = append(result, Analysis{Account: name, FullName: user.FullName, Summary: summary})
	}
	return result, nil
}

// ExecuteOrganization runs one organization's scheduled report with isolated receipts.
func ExecuteOrganization(ctx context.Context, orgID int64, now time.Time) (dailyreport.RunResult, error) {
	var empty dailyreport.RunResult
	if _, err := org_model.GetOrgByID(ctx, orgID); err != nil {
		return empty, err
	}
	locked, release, err := globallock.TryLock(ctx, fmt.Sprintf("daily_report_org_%d", orgID))
	if err != nil {
		return empty, errors.New("cannot lock daily report")
	}
	if !locked {
		return empty, errors.New("a daily report is already running")
	}
	defer release()
	cfg, err := LoadOrgConfig(orgID)
	if err != nil {
		return empty, err
	}
	due, err := cfg.Schedule.Due(now)
	if err != nil || !due {
		return empty, err
	}
	grouped, err := groupOrganization(ctx, orgID, cfg, now, nil)
	if err != nil {
		return empty, err
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return empty, err
	}
	local := now.In(loc)
	httpClient := &http.Client{Timeout: 60 * time.Second}
	provider := dailyreport.ResponsesClient{HTTP: httpClient, BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model, OmitStore: cfg.OmitStore}
	store := dailyreport.FileDeliveryStore{Path: orgPath(orgID, "deliveries.json")}
	return dailyreport.Run(ctx, local.Format("2006-01-02"), grouped, store, provider.Summarize, func(ctx context.Context, account, summary string) error {
		user, err := user_model.GetUserByName(ctx, account)
		if err != nil {
			return errors.New("cannot read report account")
		}
		return dailyreport.SendCard(ctx, httpClient, cfg.Webhook, reportTitle(account, user.FullName, local), summary)
	})
}

// ExecuteOrganizations runs configured organizations independently.
func ExecuteOrganizations(ctx context.Context, now time.Time) error {
	entries, err := os.ReadDir(filepath.Join(setting.AppDataPath, "daily-report", "organizations"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		id, err := strconv.ParseInt(entry.Name(), 10, 64)
		if err != nil || id <= 0 || !entry.IsDir() || strconv.FormatInt(id, 10) != entry.Name() {
			continue
		}
		if _, err := ExecuteOrganization(ctx, id, now); err != nil {
			failures = append(failures, fmt.Errorf("organization %d: %w", id, err))
		}
	}
	return errors.Join(failures...)
}
