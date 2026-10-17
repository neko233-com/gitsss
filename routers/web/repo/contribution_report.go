// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"cmp"
	"errors"
	"net/http"
	"time"

	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/services/context"
	repository_service "github.com/neko233-com/gitsss/services/repository"
)

func ContributionReport(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("contribution.title")
	ctx.Data["PageIsActivity"] = true
	ctx.Data["PageIsContributionReport"] = true
	ctx.HTML(http.StatusOK, templates.TplName("repo/contribution_report"))
}

func ContributionReportData(ctx *context.Context) {
	zoneName := ctx.FormString("timezone")
	if zoneName == "" {
		zoneName = "UTC"
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "invalid timezone")
		return
	}
	today := time.Now().In(zone).Format(time.DateOnly)
	to, err := time.ParseInLocation(time.DateOnly, cmp.Or(ctx.FormString("to"), today), zone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "invalid to")
		return
	}
	from, err := time.ParseInLocation(time.DateOnly, cmp.Or(ctx.FormString("from"), to.AddDate(0, 0, -29).Format(time.DateOnly)), zone)
	to = to.AddDate(0, 0, 1)
	if err != nil || !from.Before(to) || to.After(from.AddDate(0, 0, 366)) {
		ctx.JSON(http.StatusBadRequest, "choose a range within 366 days")
		return
	}
	report, err := repository_service.GetContributionReport(ctx, ctx.Cache, ctx.Repo.GitRepo, ctx.Repo.Repository.DefaultBranch, from, to, zone)
	if errors.Is(err, repository_service.ErrContributionBusy) {
		ctx.Resp.Header().Set("Retry-After", "3")
		ctx.JSON(http.StatusTooManyRequests, "statistics workers busy")
		return
	}
	if err != nil {
		ctx.ServerError("ContributionReport", err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-cache")
	ctx.JSON(http.StatusOK, report)
}
