// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/templates"
	shared_user "github.com/neko233-com/gitsss/routers/web/shared/user"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/softwarecenter"
)

func SoftwareCenter(ctx *context.Context) {
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("OrganizationHeader", err)
		return
	}
	ctx.Data["Title"] = ctx.Locale.TrString("software.title")
	ctx.Data["PageIsSoftwareCenter"] = true
	labels := map[string]string{}
	for _, key := range []string{"drop", "upload_folder", "move", "move_here", "selected", "select", "select_all", "clear", "download", "download_selected", "delete_selected", "delete_many_confirm", "transfers", "uploading", "downloading", "paused", "complete", "failed", "queued", "preparing", "validating", "pause", "resume", "retry", "choose_original", "cancel_upload", "remove_record", "resume_mismatch", "browser_downloads", "folder_hint", "empty_action", "destination", "up", "transfer_limit", "pause_navigation", "download_started", "clear_completed", "pause_all", "resume_all", "showing_limit", "tree_limit", "invalid", "caching", "cache_unavailable", "cache_required", "cache_full", "other_tab", "ready", "saved", "save_file", "confirm_saved", "download_again", "cancel_transfer", "download_changed", "session_expired"} {
		labels[key] = ctx.Locale.TrString("software." + key)
	}
	ctx.Data["SoftwareStrings"] = labels
	ctx.HTML(http.StatusOK, templates.TplName("org/software_center"))
}

func softwareError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		ctx.NotFound(nil)
	case errors.Is(err, softwarecenter.ErrInvalid):
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": ctx.Locale.TrString("software.invalid")})
	case errors.Is(err, softwarecenter.ErrConflict):
		ctx.JSON(http.StatusConflict, map[string]string{"message": ctx.Locale.TrString("software.conflict")})
	case errors.Is(err, softwarecenter.ErrQuota):
		ctx.JSON(http.StatusRequestEntityTooLarge, map[string]string{"message": ctx.Locale.TrString("software.quota_error")})
	case errors.Is(err, softwarecenter.ErrBusy):
		ctx.JSON(http.StatusTooManyRequests, map[string]string{"message": ctx.Locale.TrString("software.busy")})
	case errors.Is(err, softwarecenter.ErrDiskCapacity):
		ctx.JSON(http.StatusInsufficientStorage, map[string]string{"message": ctx.Locale.TrString("software.disk_full")})
	default:
		ctx.ServerError("SoftwareCenter", err)
	}
}

func SoftwareList(ctx *context.Context) {
	s, err := softwarecenter.Snapshot(ctx, ctx.Org.Organization.ID)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	parent, query := ctx.FormString("parent"), strings.ToLower(ctx.FormTrim("q"))
	if parent != "" && (s.Nodes[parent] == nil || !s.Nodes[parent].Directory) {
		ctx.NotFound(nil)
		return
	}
	entries, folders := []*softwarecenter.Node{}, []*softwarecenter.Node{}
	for _, node := range s.Nodes {
		if node.Directory {
			folders = append(folders, node)
		}
		if query != "" {
			if strings.Contains(strings.ToLower(node.Name), query) {
				entries = append(entries, node)
			}
		} else if node.Parent == parent {
			entries = append(entries, node)
		}
	}
	less := func(a, b *softwarecenter.Node) bool {
		if a.Directory != b.Directory {
			return a.Directory
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}
	sort.Slice(entries, func(i, j int) bool { return less(entries[i], entries[j]) })
	sort.Slice(folders, func(i, j int) bool { return less(folders[i], folders[j]) })
	breadcrumbs := []*softwarecenter.Node{}
	for id := parent; id != ""; id = s.Nodes[id].Parent {
		breadcrumbs = append(breadcrumbs, s.Nodes[id])
		if len(breadcrumbs) > 32 {
			softwareError(ctx, softwarecenter.ErrInvalid)
			return
		}
	}
	total := len(entries)
	page := min(max(ctx.FormInt("page"), 1), max((total+199)/200, 1))
	start := min((page-1)*200, total)
	end := min(start+200, total)
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, map[string]any{"entries": entries[start:end], "folders": folders, "breadcrumbs": breadcrumbs, "used": s.Used(), "reserved": s.Reserved(), "quota": s.Quota, "total": total, "page": page})
}

func SoftwareMutation(ctx *context.Context) {
	orgID := ctx.Org.Organization.ID
	var err error
	switch ctx.PathParam("action") {
	case "quota":
		if !ctx.Org.IsOwner {
			ctx.NotFound(nil)
			return
		}
		quota, parseErr := strconv.ParseInt(ctx.FormString("gib"), 10, 64)
		if parseErr != nil || quota < 1 || quota > 10240 {
			softwareError(ctx, softwarecenter.ErrQuota)
			return
		}
		err = softwarecenter.SetQuota(ctx, orgID, quota*1024*1024*1024)
	case "mkdir":
		var node *softwarecenter.Node
		node, err = softwarecenter.CreateFolder(ctx, orgID, ctx.Doer.ID, ctx.FormString("parent"), ctx.FormString("name"))
		if err == nil {
			ctx.JSON(http.StatusCreated, node)
			return
		}
	case "rename":
		err = softwarecenter.Rename(ctx, orgID, ctx.FormString("id"), ctx.FormString("name"))
	case "delete":
		err = softwarecenter.Delete(ctx, orgID, ctx.FormString("id"))
	default:
		ctx.NotFound(nil)
		return
	}
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func SoftwareUpload(ctx *context.Context) {
	size, err := strconv.ParseInt(ctx.FormString("size"), 10, 64)
	if err != nil || size < 0 || size > softwarecenter.MaxQuota {
		softwareError(ctx, softwarecenter.ErrQuota)
		return
	}
	if ctx.Req.ContentLength >= 0 && ctx.Req.ContentLength != size {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	body := http.MaxBytesReader(ctx.Resp, ctx.Req.Body, size+1)
	defer body.Close()
	node, err := softwarecenter.Upload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.FormString("parent"), ctx.FormString("name"), size, body)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, node)
}

func SoftwareDownload(ctx *context.Context) {
	node, file, err := softwarecenter.Open(ctx, ctx.Org.Organization.ID, ctx.PathParam("id"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	defer file.Close()
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	ctx.Resp.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": node.Name}))
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	ctx.Resp.Header().Set("ETag", `"sha256-`+node.Digest+`"`)
	ctx.Resp.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(ctx.Resp, ctx.Req, node.Name, time.Unix(node.Modified, 0), file)
}
