// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/lfsgc"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/forms"
	repo_service "github.com/neko233-com/gitsss/services/repository"
)

const tplLFSGC templates.TplName = "admin/lfs_gc"

// LFSGC shows the unreferenced LFS object scan.
func LFSGC(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.lfs_gc")
	ctx.Data["PageIsAdminLFSGC"] = true
	ctx.Data["OlderThanHours"] = int(lfsgc.DefaultAge / time.Hour)
	ctx.HTML(http.StatusOK, tplLFSGC)
}

// LFSGCPost scans or deletes unreferenced LFS objects.
func LFSGCPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminLFSGCForm)
	ctx.Data["Title"] = ctx.Tr("admin.lfs_gc")
	ctx.Data["PageIsAdminLFSGC"] = true
	hours := form.OlderThanHours
	if hours <= 0 {
		hours = int(lfsgc.DefaultAge / time.Hour)
	}
	ctx.Data["OlderThanHours"] = hours
	ctx.Data["Repository"] = form.Repository
	apply := form.Op == "apply"
	if apply && !form.Confirm {
		ctx.Flash.Error(ctx.Tr("admin.lfs_gc.confirm_required"), true)
		ctx.HTML(http.StatusBadRequest, tplLFSGC)
		return
	}
	report, err := repo_service.CollectLFSGarbage(ctx, lfsRequest(form, apply))
	if wantsJSON(ctx, form.Format) {
		status := http.StatusOK
		if err != nil {
			status = http.StatusBadRequest
			ctx.JSON(status, map[string]any{"error": err.Error(), "report": report})
			return
		}
		ctx.JSON(status, report)
		return
	}
	ctx.Data["Report"] = report
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
		ctx.HTML(http.StatusBadRequest, tplLFSGC)
		return
	}
	if apply {
		ctx.Flash.Success(ctx.Tr("admin.lfs_gc.applied", report.Deleted), true)
	} else {
		ctx.Flash.Success(ctx.Tr("admin.lfs_gc.scanned", report.Orphaned), true)
	}
	ctx.HTML(http.StatusOK, tplLFSGC)
}

func lfsRequest(form *forms.AdminLFSGCForm, apply bool) lfsgc.Request {
	hours := form.OlderThanHours
	if hours <= 0 {
		hours = int(lfsgc.DefaultAge / time.Hour)
	}
	return lfsgc.Request{
		Apply:      apply,
		OlderThan:  time.Now().Add(-time.Duration(hours) * time.Hour),
		Repository: strings.TrimSpace(form.Repository),
	}
}

func wantsJSON(ctx *context.Context, format string) bool {
	return format == "json" || strings.Contains(ctx.Req.Header.Get("Accept"), "application/json")
}
