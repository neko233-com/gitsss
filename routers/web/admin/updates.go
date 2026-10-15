// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strings"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/updatechecker"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/forms"
)

const tplUpdates templates.TplName = "admin/updates"

// Updates shows the GitHub release check and mirror list.
func Updates(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.updates")
	ctx.Data["PageIsAdminUpdates"] = true
	opts := updatechecker.OptionsFromConfig()
	if !updateSectionHasKey("CHECK_GITHUB") {
		// The scheduled task stays off. The form offers a GitHub check until an admin saves a choice.
		opts.CheckGitHub = true
	}
	prepareUpdateData(ctx, opts, "", scheduleEnabled())
	if report, ok := updatechecker.LoadReport(ctx); ok {
		ctx.Data["Report"] = report
	}
	ctx.HTML(http.StatusOK, tplUpdates)
}

// UpdatesPost saves mirror settings or runs a check.
func UpdatesPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminUpdateForm)
	opts := optionsFromAdminForm(form)
	ctx.Data["Title"] = ctx.Tr("admin.updates")
	ctx.Data["PageIsAdminUpdates"] = true

	if form.Op == "save" {
		enabled := form.Enabled
		if err := updatechecker.SaveSettings(updatechecker.Settings{
			Mirrors:     opts.Mirrors,
			GitHubRepo:  opts.GitHubRepo,
			CheckGitHub: opts.CheckGitHub,
			Enabled:     &enabled,
		}); err != nil {
			ctx.Flash.Error(err.Error(), true)
			prepareUpdateData(ctx, opts, form.Mirrors, form.Enabled)
			ctx.HTML(http.StatusBadRequest, tplUpdates)
			return
		}
		ctx.Flash.Success(ctx.Tr("admin.updates.saved"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/updates")
		return
	}

	opts.CurrentVersion = setting.AppVer
	if opts.GitHubRepo == "" {
		opts.GitHubRepo = updatechecker.DefaultGitHubRepo
	}
	report, err := updatechecker.Check(ctx, opts)
	if err == nil && report.LatestVersion != "" {
		if saveErr := updatechecker.UpdateRemoteVersion(ctx, report.LatestVersion); saveErr != nil && report.Note == "" {
			report.Note = saveErr.Error()
		}
	}
	_ = updatechecker.SaveReport(ctx, report)
	if form.Format == "json" || strings.Contains(ctx.Req.Header.Get("Accept"), "application/json") {
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadGateway
		}
		ctx.JSON(status, report)
		return
	}
	prepareUpdateData(ctx, opts, form.Mirrors, form.Enabled)
	ctx.Data["Report"] = report
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
	} else if report.NeedUpdate {
		ctx.Flash.Success(ctx.Tr("admin.updates.available", report.LatestVersion), true)
	} else {
		ctx.Flash.Success(ctx.Tr("admin.updates.up_to_date"), true)
	}
	ctx.HTML(http.StatusOK, tplUpdates)
}

func optionsFromAdminForm(form *forms.AdminUpdateForm) updatechecker.Options {
	mirrors, _ := updatechecker.ParseMirrors(form.Mirrors)
	return updatechecker.Options{
		HTTPEndpoint: updatechecker.OptionsFromConfig().HTTPEndpoint,
		CheckGitHub:  form.CheckGitHub,
		GitHubRepo:   strings.TrimSpace(form.GitHubRepo),
		Mirrors:      mirrors,
	}
}

func prepareUpdateData(ctx *context.Context, opts updatechecker.Options, mirrorText string, schedule bool) {
	if mirrorText == "" {
		mirrorText = updatechecker.MirrorText(opts.Mirrors)
	}
	repo := opts.GitHubRepo
	if repo == "" {
		repo = updatechecker.DefaultGitHubRepo
	}
	ctx.Data["MirrorsText"] = mirrorText
	ctx.Data["GitHubRepo"] = repo
	ctx.Data["CheckGitHub"] = opts.CheckGitHub
	ctx.Data["ScheduleEnabled"] = schedule
	ctx.Data["DefaultMirror"] = updatechecker.DefaultMirror
	ctx.Data["CurrentVersion"] = setting.AppVer
}

func scheduleEnabled() bool {
	if !updateSectionHasKey("ENABLED") {
		return false
	}
	return setting.CfgProvider.Section("cron.update_checker").Key("ENABLED").MustBool(false)
}

func updateSectionHasKey(name string) bool {
	if setting.CfgProvider == nil {
		return false
	}
	return setting.CfgProvider.Section("cron.update_checker").HasKey(name)
}
