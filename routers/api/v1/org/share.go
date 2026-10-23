// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/neko233-com/gitsss/models/organization"
	share_model "github.com/neko233-com/gitsss/models/orgshare"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/orgshare"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
	share_service "github.com/neko233-com/gitsss/services/orgshare"
)

type shareJSON struct {
	ID          int64  `json:"id"`
	ParentID    int64  `json:"parent_id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	ExpiresUnix int64  `json:"expires_unix"`
	Permanent   bool   `json:"permanent"`
	Link        string `json:"link"`
}

type uploadJSON struct {
	ID         string          `json:"id"`
	Size       int64           `json:"size"`
	Received   int64           `json:"received"`
	Missing    []orgshare.Span `json:"missing"`
	ChunkBytes int64           `json:"chunk_bytes"`
	Parallel   int             `json:"parallel"`
}

func shareLink(ctx *context.APIContext, id int64) string {
	return ctx.Org.Organization.OrganisationLink() + "/share/files/" + strconv.FormatInt(id, 10)
}

func toShareJSON(ctx *context.APIContext, item *share_model.ShareItem) shareJSON {
	return shareJSON{
		ID: item.ID, ParentID: item.ParentID, Name: item.Name, Kind: item.Kind, Size: item.Size,
		ExpiresUnix: int64(item.ExpiresUnix), Permanent: item.ExpiresUnix == 0, Link: shareLink(ctx, item.ID),
	}
}

func shareActor(ctx *context.APIContext) (owner, ok bool) {
	if !setting.OrgShare.Enabled {
		ctx.APIError(http.StatusNotFound, "organization share is unavailable")
		return false, false
	}
	if ctx.Doer == nil {
		ctx.APIError(http.StatusUnauthorized, "token is required")
		return false, false
	}
	if ctx.Doer.IsAdmin {
		return true, true
	}
	isOwner, err := organization.IsOrganizationOwner(ctx, ctx.Org.Organization.ID, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return false, false
	}
	isMember := isOwner
	if !isMember {
		isMember, err = organization.IsOrganizationMember(ctx, ctx.Org.Organization.ID, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return false, false
		}
	}
	if !isMember {
		ctx.APIError(http.StatusForbidden, "organization membership is required")
		return false, false
	}
	return isOwner, true
}

func writeShareLimit(ctx *context.APIContext, err error) bool {
	var limit *orgshare.LimitError
	if !errors.As(err, &limit) {
		return false
	}
	status := http.StatusRequestEntityTooLarge
	if limit.Code == "disk" || limit.Code == "disk_unknown" {
		status = http.StatusInsufficientStorage
	}
	ctx.JSON(status, map[string]any{
		"message": limit.Error(), "code": limit.Code,
		"used_bytes": limit.Used, "limit_bytes": limit.Limit, "incoming_bytes": limit.Incoming,
		"available_bytes": limit.Available, "reserve_bytes": limit.Reserve,
	})
	return true
}

func toUploadJSON(upload *share_model.ShareUpload) (uploadJSON, error) {
	missing, err := share_service.UploadMissing(upload)
	if err != nil {
		return uploadJSON{}, err
	}
	return uploadJSON{
		ID: upload.ID, Size: upload.Size, Received: upload.Received, Missing: missing,
		ChunkBytes: orgshare.DefaultChunkBytes, Parallel: orgshare.DefaultParallel,
	}, nil
}

// ListShares returns one folder.
func ListShares(ctx *context.APIContext) {
	if _, ok := shareActor(ctx); !ok {
		return
	}
	items, err := share_service.List(ctx, ctx.Org.Organization.ID, ctx.FormInt64("parent"))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	result := make([]shareJSON, 0, len(items))
	for _, item := range items {
		result = append(result, toShareJSON(ctx, item))
	}
	ctx.JSON(http.StatusOK, result)
}

// CreateShareText stores a note.
func CreateShareText(ctx *context.APIContext) {
	if _, ok := shareActor(ctx); !ok {
		return
	}
	var body struct {
		Name      string `json:"name"`
		Body      string `json:"body"`
		ParentID  int64  `json:"parent_id"`
		Permanent bool   `json:"permanent"`
	}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil {
		ctx.APIError(http.StatusBadRequest, "invalid JSON body")
		return
	}
	item, err := share_service.CreateText(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, body.ParentID, body.Name, body.Body, body.Permanent)
	if writeShareLimit(ctx, err) {
		return
	}
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	ctx.JSON(http.StatusCreated, toShareJSON(ctx, item))
}

// BeginShareUpload starts a resumable upload after quota and disk checks.
func BeginShareUpload(ctx *context.APIContext) {
	if _, ok := shareActor(ctx); !ok {
		return
	}
	var body struct {
		Name      string `json:"name"`
		Size      int64  `json:"size"`
		ParentID  int64  `json:"parent_id"`
		Permanent bool   `json:"permanent"`
		Resume    bool   `json:"resume"`
	}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil {
		ctx.APIError(http.StatusBadRequest, "invalid JSON body")
		return
	}
	upload, err := share_service.BeginUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, body.ParentID, body.Name, body.Size, body.Permanent, body.Resume)
	if writeShareLimit(ctx, err) {
		return
	}
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	payload, err := toUploadJSON(upload)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, payload)
}

// GetShareUpload returns ranges that are still missing.
func GetShareUpload(ctx *context.APIContext) {
	if _, ok := shareActor(ctx); !ok {
		return
	}
	upload, err := share_model.GetUpload(ctx, ctx.Org.Organization.ID, ctx.PathParam("id"))
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	payload, err := toUploadJSON(upload)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, payload)
}

// AppendShareUpload writes one chunk. Clients may send several chunks at once.
func AppendShareUpload(ctx *context.APIContext) {
	owner, ok := shareActor(ctx)
	if !ok {
		return
	}
	upload, err := share_model.GetUpload(ctx, ctx.Org.Organization.ID, ctx.PathParam("id"))
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	if upload.UploaderID != ctx.Doer.ID && !owner {
		ctx.APIError(http.StatusForbidden, "only the uploader can continue this upload")
		return
	}
	span, err := orgshare.ParseContentRange(ctx.Req.Header.Get("Content-Range"))
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	if span.Length() > orgshare.MaxChunkBytes {
		ctx.APIError(http.StatusBadRequest, "chunk is larger than 32 MiB")
		return
	}
	if span.Total != upload.Size {
		ctx.APIError(http.StatusBadRequest, "content range total does not match the upload")
		return
	}
	updated, err := share_service.Append(ctx, ctx.Org.Organization.ID, upload.ID, span.Start, io.LimitReader(ctx.Req.Body, span.Length()), span.Length())
	if writeShareLimit(ctx, err) {
		return
	}
	if err != nil {
		ctx.APIError(http.StatusConflict, err.Error())
		return
	}
	payload, err := toUploadJSON(updated)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, payload)
}

// CompleteShareUpload finishes a resumable upload.
func CompleteShareUpload(ctx *context.APIContext) {
	owner, ok := shareActor(ctx)
	if !ok {
		return
	}
	upload, err := share_model.GetUpload(ctx, ctx.Org.Organization.ID, ctx.PathParam("id"))
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	if upload.UploaderID != ctx.Doer.ID && !owner {
		ctx.APIError(http.StatusForbidden, "only the uploader can finish this upload")
		return
	}
	item, err := share_service.Complete(ctx, ctx.Org.Organization.ID, upload.ID)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	ctx.JSON(http.StatusCreated, toShareJSON(ctx, item))
}

// GetShare streams a file and honors a single HTTP Range per request.
func GetShare(ctx *context.APIContext) {
	if _, ok := shareActor(ctx); !ok {
		return
	}
	id, err := strconv.ParseInt(ctx.PathParam("id"), 10, 64)
	if err != nil {
		ctx.APIError(http.StatusNotFound, "share item not found")
		return
	}
	item, file, err := share_service.Open(ctx, ctx.Org.Organization.ID, id)
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	if item.Kind == share_model.KindText {
		ctx.PlainText(http.StatusOK, item.Text)
		return
	}
	if file == nil {
		ctx.APIError(http.StatusBadRequest, "folders are listed, not downloaded")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	ctx.Resp.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(item.Name))
	ctx.Resp.Header().Set("Accept-Ranges", "bytes")
	ctx.Resp.Header().Set("No-Gzip-Compression", "1")
	http.ServeContent(ctx.Resp, ctx.Req, item.Name, info.ModTime(), file)
}

// UpdateShare marks one object permanent or restores the default lifetime.
func UpdateShare(ctx *context.APIContext) {
	owner, ok := shareActor(ctx)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(ctx.PathParam("id"), 10, 64)
	if err != nil {
		ctx.APIError(http.StatusNotFound, "share item not found")
		return
	}
	item, err := share_model.GetItem(ctx, ctx.Org.Organization.ID, id)
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	if !orgshare.CanDelete(owner, item.UploaderID == ctx.Doer.ID) {
		ctx.APIError(http.StatusForbidden, "only the uploader or an organization owner can change this share")
		return
	}
	var body struct {
		Permanent bool `json:"permanent"`
	}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil {
		ctx.APIError(http.StatusBadRequest, "invalid JSON body")
		return
	}
	updated, err := share_service.SetPermanent(ctx, ctx.Org.Organization.ID, id, body.Permanent)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, toShareJSON(ctx, updated))
}

// DeleteShare removes one object.
func DeleteShare(ctx *context.APIContext) {
	owner, ok := shareActor(ctx)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(ctx.PathParam("id"), 10, 64)
	if err != nil {
		ctx.APIError(http.StatusNotFound, "share item not found")
		return
	}
	item, err := share_model.GetItem(ctx, ctx.Org.Organization.ID, id)
	if err != nil {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	if !orgshare.CanDelete(owner, item.UploaderID == ctx.Doer.ID) {
		ctx.APIError(http.StatusForbidden, "only the uploader or an organization owner can delete this share")
		return
	}
	if err := share_service.Delete(ctx, ctx.Org.Organization.ID, id); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ClearShares removes the organization share. Owners only.
func ClearShares(ctx *context.APIContext) {
	owner, ok := shareActor(ctx)
	if !ok {
		return
	}
	if !owner {
		ctx.APIError(http.StatusForbidden, "organization owner is required")
		return
	}
	if err := share_service.Clear(ctx, ctx.Org.Organization.ID); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
