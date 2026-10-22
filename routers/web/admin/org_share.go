// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/services/context"
	share_service "github.com/neko233-com/gitsss/services/orgshare"
)

const tplOrgShare templates.TplName = "admin/org_share"

type orgNameRow struct {
	ID   int64
	Name string
}

// OrgShare shows disk space and each organization's share quota.
func OrgShare(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.org_share")
	ctx.Data["PageIsAdminOrgShare"] = true
	report, err := loadOrgShareReport(ctx)
	if err != nil {
		ctx.ServerError("OrgShareReport", err)
		return
	}
	ctx.Data["Report"] = report
	ctx.Data["DefaultQuota"] = setting.OrgShare.MaxBytes
	ctx.HTML(http.StatusOK, tplOrgShare)
}

// OrgSharePost sets one organization's quota. Zero restores the 100 GB default.
func OrgSharePost(ctx *context.Context) {
	orgID := ctx.FormInt64("org_id")
	raw := strings.TrimSpace(ctx.FormString("max_gb"))
	var maxBytes int64
	if raw != "" && raw != "0" {
		gb, err := strconv.ParseFloat(raw, 64)
		if err != nil || gb < 0 {
			ctx.Flash.Error(ctx.Tr("admin.org_share.invalid"), true)
			ctx.Redirect(setting.AppSubURL + "/-/admin/org-share")
			return
		}
		maxBytes = int64(gb * float64(1<<30))
	}
	if err := share_service.SetQuota(ctx, orgID, maxBytes); err != nil {
		ctx.ServerError("SetQuota", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("admin.org_share.saved"))
	ctx.Redirect(setting.AppSubURL + "/-/admin/org-share")
}

func loadOrgShareReport(ctx *context.Context) (*share_service.DiskReport, error) {
	rows := make([]orgNameRow, 0)
	if err := db.GetEngine(ctx).Table("user").Where("type = ?", user_model.UserTypeOrganization).Cols("id", "name").Asc("lower_name").Find(&rows); err != nil {
		return nil, err
	}
	usages := make([]share_service.Usage, 0, len(rows))
	for _, row := range rows {
		usage, err := share_service.FillUsage(ctx, row.ID, row.Name)
		if err != nil {
			return nil, err
		}
		usages = append(usages, usage)
	}
	return share_service.Report(ctx, usages)
}
