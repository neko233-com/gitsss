// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/services/context"
	dailyreport_service "github.com/neko233-com/gitsss/services/dailyreport"
)

type dailyReportRole struct {
	Name     string
	Selected bool
}

func reportRoles(ctx *context.Context, rows []dailyReportAccount, cfg dailyreport.Config) ([]dailyReportRole, error) {
	labels := make(map[string]string)
	selected := make(map[string]bool)
	for _, label := range cfg.RoleLabels {
		key := strings.ToLower(label)
		labels[key] = label
		selected[key] = true
	}
	for _, row := range rows {
		values, err := user_model.GetUserSettingJSON(ctx, row.ID, user_model.SettingsKeyRoleLabels, []string{})
		if err != nil {
			return nil, err
		}
		for _, label := range values {
			key := strings.ToLower(label)
			if _, exists := labels[key]; !exists {
				labels[key] = label
			}
		}
	}
	roles := make([]dailyReportRole, 0, len(labels))
	for key, label := range labels {
		roles = append(roles, dailyReportRole{Name: label, Selected: selected[key]})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles, nil
}

type dailyReportAccount struct {
	ID                   int64
	Name, FullName, Kind string
}

func reportAccounts(ctx *context.Context, cfg dailyreport.Config) ([]dailyReportAccount, error) {
	users, err := dailyreport_service.OrgMembers(ctx, ctx.Org.Organization.ID)
	if err != nil {
		return nil, err
	}
	rows := make([]dailyReportAccount, 0, len(users))
	for _, user := range users {
		rows = append(rows, dailyReportAccount{user.ID, user.Name, user.FullName, cfg.Accounts[user.Name]})
	}
	return rows, nil
}

func renderDailyReport(ctx *context.Context, cfg dailyreport.Config, status int) {
	rows, err := reportAccounts(ctx, cfg)
	if err != nil {
		ctx.ServerError("DailyReportAccounts", err)
		return
	}
	roles, err := reportRoles(ctx, rows, cfg)
	if err != nil {
		ctx.ServerError("DailyReportRoles", err)
		return
	}
	if ctx.Data["ReportManualAccounts"] == nil {
		ctx.Data["ReportManualAccounts"] = map[int64]bool{}
	}
	if ctx.Data["ReportManualRoles"] == nil {
		ctx.Data["ReportManualRoles"] = map[string]bool{}
	}
	ctx.Data["ReportRoles"] = roles
	ctx.Data["ReportRoleFilterEnabled"] = cfg.RoleFilterEnabled
	years := make([]string, 0, len(cfg.Schedule.CalendarYears))
	for _, year := range cfg.Schedule.CalendarYears {
		years = append(years, strconv.Itoa(year))
	}
	ctx.Data["Title"] = ctx.Tr("admin.daily_report")
	ctx.Data["PageIsOrgDailyReport"] = true
	ctx.Data["ReportSchedule"] = cfg.Schedule
	ctx.Data["ReportBaseURL"] = cfg.BaseURL
	ctx.Data["ReportModel"] = cfg.Model
	ctx.Data["ReportOmitStore"] = cfg.OmitStore
	ctx.Data["ReportHasKey"] = cfg.APIKey != ""
	ctx.Data["ReportHasWebhook"] = cfg.Webhook != ""
	ctx.Data["ReportYears"] = strings.Join(years, ", ")
	ctx.Data["ReportHolidays"] = strings.Join(cfg.Schedule.Holidays, "\n")
	ctx.Data["ReportWorkdays"] = strings.Join(cfg.Schedule.Workdays, "\n")
	ctx.Data["ReportAccounts"] = rows
	ctx.HTML(status, templates.TplName("org/settings/daily_report"))
}

// DailyReport displays private settings without echoing credentials.
func DailyReport(ctx *context.Context) {
	cfg, err := dailyreport_service.LoadOrgConfig(ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("DailyReportConfig", err)
		return
	}
	renderDailyReport(ctx, cfg, http.StatusOK)
}

func reportLines(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t' })
}

// DailyReportPost validates and saves settings; blank secret fields retain their values.
func DailyReportPost(ctx *context.Context) {
	cfg, err := dailyreport_service.LoadOrgConfig(ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("DailyReportConfig", err)
		return
	}
	rows, err := reportAccounts(ctx, cfg)
	if err != nil {
		ctx.ServerError("DailyReportAccounts", err)
		return
	}
	cfg.BaseURL = strings.TrimSpace(ctx.FormString("base_url"))
	cfg.Model = strings.TrimSpace(ctx.FormString("model"))
	if value := strings.TrimSpace(ctx.FormString("api_key")); value != "" {
		cfg.APIKey = value
	}
	if value := strings.TrimSpace(ctx.FormString("webhook")); value != "" {
		cfg.Webhook = value
	}
	if ctx.FormBool("clear_api_key") {
		cfg.APIKey = ""
	}
	if ctx.FormBool("clear_webhook") {
		cfg.Webhook = ""
	}
	if mode := ctx.FormString("role_filter_mode"); mode != "" {
		switch mode {
		case "all":
			cfg.RoleFilterEnabled = false
			cfg.RoleLabels = nil
		case "selected":
			cfg.RoleFilterEnabled = true
			cfg.RoleLabels = ctx.FormStrings("role_filter")
		default:
			err = errors.New("invalid role filter mode")
		}
	}
	cfg.OmitStore = ctx.FormBool("omit_store")
	cfg.Schedule.Enabled = ctx.FormBool("enabled")
	cfg.Schedule.Time = strings.TrimSpace(ctx.FormString("time"))
	cfg.Schedule.SkipHolidays = ctx.FormBool("skip_holidays")
	cfg.Schedule.Holidays = reportLines(ctx.FormString("holidays"))
	cfg.Schedule.Workdays = reportLines(ctx.FormString("workdays"))
	cfg.Schedule.CalendarYears = nil
	for _, value := range reportLines(ctx.FormString("years")) {
		year, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			err = errors.New("invalid calendar year")
			break
		}
		cfg.Schedule.CalendarYears = append(cfg.Schedule.CalendarYears, year)
	}
	cfg.Accounts = make(map[string]string)
	for _, row := range rows {
		kind := ctx.FormString("account_" + strconv.FormatInt(row.ID, 10))
		if kind != "" {
			cfg.Accounts[row.Name] = kind
		}
	}
	if err == nil {
		err = dailyreport_service.SaveOrgConfig(ctx.Org.Organization.ID, cfg)
	}
	if err != nil {
		ctx.Flash.Error(ctx.Tr("admin.daily_report.invalid"), true)
		renderDailyReport(ctx, cfg, http.StatusBadRequest)
		return
	}
	ctx.Flash.Success(ctx.Tr("admin.daily_report.saved"))
	ctx.Redirect(ctx.Org.Organization.OrganisationLink() + "/settings/daily-report")
}

// DailyReportAnalyze previews explicitly selected organization members and roles.
func DailyReportAnalyze(ctx *context.Context) {
	cfg, err := dailyreport_service.LoadOrgConfig(ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("DailyReportConfig", err)
		return
	}
	selection := dailyreport_service.ManualSelection{RoleLabels: ctx.FormStrings("manual_role")}
	selectedIDs := make(map[int64]bool)
	for _, value := range ctx.FormStrings("manual_account") {
		id, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || id <= 0 {
			ctx.Flash.Error("Invalid account selection", true)
			renderDailyReport(ctx, cfg, http.StatusBadRequest)
			return
		}
		selection.AccountIDs = append(selection.AccountIDs, id)
		selectedIDs[id] = true
	}
	selectedRoles := make(map[string]bool)
	for _, role := range selection.RoleLabels {
		selectedRoles[role] = true
	}
	ctx.Data["ReportManualAccounts"] = selectedIDs
	ctx.Data["ReportManualRoles"] = selectedRoles
	if len(selection.AccountIDs) == 0 && len(selection.RoleLabels) == 0 {
		ctx.Flash.Error(ctx.Tr("org.daily_report.selection_required"), true)
		renderDailyReport(ctx, cfg, http.StatusBadRequest)
		return
	}
	results, err := dailyreport_service.AnalyzeOrganization(ctx, ctx.Org.Organization.ID, time.Now(), selection)
	if err != nil {
		log.Error("Organization daily report analysis failed: %v", err)
		ctx.Flash.Error(err.Error(), true)
		renderDailyReport(ctx, cfg, http.StatusBadRequest)
		return
	}
	ctx.Data["ReportAnalyzed"] = true
	ctx.Data["ReportAnalyses"] = results
	renderDailyReport(ctx, cfg, http.StatusOK)
}
