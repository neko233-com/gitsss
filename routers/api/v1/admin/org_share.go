// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
	share_service "github.com/neko233-com/gitsss/services/orgshare"
)

type shareOrgRow struct {
	ID   int64
	Name string
}

// GetOrgShare returns disk space and per-organization usage.
func GetOrgShare(ctx *context.APIContext) {
	rows := make([]shareOrgRow, 0)
	if err := db.GetEngine(ctx).Table("user").Where("type = ?", user_model.UserTypeOrganization).Cols("id", "name").Asc("lower_name").Find(&rows); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	usages := make([]share_service.Usage, 0, len(rows))
	for _, row := range rows {
		usage, err := share_service.FillUsage(ctx, row.ID, row.Name)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		usages = append(usages, usage)
	}
	report, err := share_service.Report(ctx, usages)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"default_quota_bytes": setting.OrgShare.MaxBytes, "report": report})
}

// PutOrgShareQuota sets one organization's cap. max_bytes 0 restores the default.
func PutOrgShareQuota(ctx *context.APIContext) {
	var body struct {
		Org      string `json:"org"`
		OrgID    int64  `json:"org_id"`
		MaxBytes int64  `json:"max_bytes"`
	}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil {
		ctx.APIError(http.StatusBadRequest, "invalid JSON body")
		return
	}
	orgID := body.OrgID
	if body.Org != "" {
		org, err := organization.GetOrgByName(ctx, body.Org)
		if err != nil {
			ctx.APIError(http.StatusNotFound, err.Error())
			return
		}
		orgID = org.ID
	}
	if orgID <= 0 {
		ctx.APIError(http.StatusBadRequest, "org or org_id is required")
		return
	}
	if body.MaxBytes < 0 {
		ctx.APIError(http.StatusBadRequest, "max_bytes must not be negative")
		return
	}
	if err := share_service.SetQuota(ctx, orgID, body.MaxBytes); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	usage, err := share_service.FillUsage(ctx, orgID, body.Org)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, usage)
}
