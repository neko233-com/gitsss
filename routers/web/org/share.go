// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"io"
	"net/http"
	"strconv"

	share_model "github.com/neko233-com/gitsss/models/orgshare"
	"github.com/neko233-com/gitsss/modules/orgshare"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	shared_user "github.com/neko233-com/gitsss/routers/web/shared/user"
	"github.com/neko233-com/gitsss/services/context"
	share_service "github.com/neko233-com/gitsss/services/orgshare"
)

const tplOrgShare templates.TplName = "org/share"

// Share shows the organization file share.
func Share(ctx *context.Context) {
	if !setting.OrgShare.Enabled {
		ctx.NotFound(nil)
		return
	}
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}
	parentID := ctx.FormInt64("parent")
	items, err := share_service.List(ctx, ctx.Org.Organization.ID, parentID)
	if err != nil {
		ctx.ServerError("ListShares", err)
		return
	}
	ancestors, err := share_service.Ancestors(ctx, ctx.Org.Organization.ID, parentID)
	if err != nil && !share_model.IsErrNotExist(err) {
		ctx.ServerError("ShareAncestors", err)
		return
	}
	usage, err := share_service.FillUsage(ctx, ctx.Org.Organization.ID, ctx.Org.Organization.Name)
	if err != nil {
		ctx.ServerError("ShareUsage", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("org.share")
	ctx.Data["PageIsOrgShare"] = true
	ctx.Data["ShareItems"] = items
	ctx.Data["ShareParent"] = parentID
	ctx.Data["ShareAncestors"] = ancestors
	ctx.Data["ShareUsage"] = usage
	ctx.Data["ShareExpireDays"] = setting.OrgShare.ExpireDays
	ctx.Data["ShareChunkBytes"] = orgshare.DefaultChunkBytes
	ctx.Data["ShareParallel"] = orgshare.DefaultParallel
	ctx.Data["CanClearShares"] = ctx.Org.IsOwner
	ctx.HTML(http.StatusOK, tplOrgShare)
}

// ShareText stores a text note.
func ShareText(ctx *context.Context) {
	if !setting.OrgShare.Enabled {
		ctx.NotFound(nil)
		return
	}
	parentID := ctx.FormInt64("parent")
	if _, err := share_service.CreateText(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, parentID, ctx.FormString("name"), ctx.FormString("body"), ctx.FormString("permanent") == "1"); err != nil {
		ctx.Flash.Error(err.Error())
	}
	ctx.Redirect(shareRedirect(ctx, parentID))
}

// ShareDelete removes one object the member is allowed to delete.
func ShareDelete(ctx *context.Context) {
	if !setting.OrgShare.Enabled {
		ctx.NotFound(nil)
		return
	}
	parentID := ctx.FormInt64("parent")
	item, err := share_model.GetItem(ctx, ctx.Org.Organization.ID, ctx.FormInt64("id"))
	if err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(shareRedirect(ctx, parentID))
		return
	}
	if !orgshare.CanDelete(ctx.Org.IsOwner, item.UploaderID == ctx.Doer.ID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	if err := share_service.Delete(ctx, ctx.Org.Organization.ID, item.ID); err != nil {
		ctx.Flash.Error(err.Error())
	}
	ctx.Redirect(shareRedirect(ctx, parentID))
}

// ShareKeep marks one object permanent or restores the default lifetime.
func ShareKeep(ctx *context.Context) {
	if !setting.OrgShare.Enabled {
		ctx.NotFound(nil)
		return
	}
	parentID := ctx.FormInt64("parent")
	item, err := share_model.GetItem(ctx, ctx.Org.Organization.ID, ctx.FormInt64("id"))
	if err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(shareRedirect(ctx, parentID))
		return
	}
	if !orgshare.CanDelete(ctx.Org.IsOwner, item.UploaderID == ctx.Doer.ID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	if _, err := share_service.SetPermanent(ctx, ctx.Org.Organization.ID, item.ID, ctx.FormString("permanent") == "1"); err != nil {
		ctx.Flash.Error(err.Error())
	}
	ctx.Redirect(shareRedirect(ctx, parentID))
}

// ShareClear removes every object. Owners only.
func ShareClear(ctx *context.Context) {
	if !setting.OrgShare.Enabled || !ctx.Org.IsOwner {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	if err := share_service.Clear(ctx, ctx.Org.Organization.ID); err != nil {
		ctx.Flash.Error(err.Error())
	}
	ctx.Redirect(ctx.Org.OrgLink + "/share")
}

// ShareDownload streams one file or text note and honors HTTP Range.
func ShareDownload(ctx *context.Context) {
	if !setting.OrgShare.Enabled {
		ctx.NotFound(nil)
		return
	}
	id, err := strconv.ParseInt(ctx.PathParam("id"), 10, 64)
	if err != nil {
		ctx.NotFound(nil)
		return
	}
	item, file, err := share_service.Open(ctx, ctx.Org.Organization.ID, id)
	if err != nil {
		if share_model.IsErrNotExist(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("OpenShare", err)
		return
	}
	if item.Kind == share_model.KindText {
		ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		ctx.Resp.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(item.Name))
		_, _ = io.WriteString(ctx.Resp, item.Text)
		return
	}
	if file == nil {
		ctx.NotFound(nil)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		ctx.ServerError("StatShare", err)
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	ctx.Resp.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(item.Name))
	ctx.Resp.Header().Set("Accept-Ranges", "bytes")
	ctx.Resp.Header().Set("No-Gzip-Compression", "1")
	http.ServeContent(ctx.Resp, ctx.Req, item.Name, info.ModTime(), file)
}

func shareRedirect(ctx *context.Context, parentID int64) string {
	link := ctx.Org.OrgLink + "/share"
	if parentID > 0 {
		return link + "?parent=" + strconv.FormatInt(parentID, 10)
	}
	return link
}
