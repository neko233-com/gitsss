// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	stdcontext "context"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/softwarecenter"
)

func SoftwareBatch(ctx *context.Context) {
	if err := ctx.Req.ParseForm(); err != nil {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	var err error
	switch ctx.PathParam("operation") {
	case "move":
		err = softwarecenter.Move(ctx, ctx.Org.Organization.ID, ctx.FormStrings("ids"), ctx.FormString("parent"))
	case "delete":
		err = softwarecenter.DeleteMany(ctx, ctx.Org.Organization.ID, ctx.FormStrings("ids"))
	case "directories":
		var result map[string]string
		result, err = softwarecenter.EnsureDirectories(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.FormString("parent"), ctx.FormStrings("paths"))
		if err == nil {
			ctx.JSON(http.StatusCreated, result)
			return
		}
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

func SoftwareUploads(ctx *context.Context) {
	items, err := softwarecenter.Uploads(ctx, ctx.Org.Organization.ID, ctx.Doer.ID)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, items)
}

func SoftwareUploadBegin(ctx *context.Context) {
	size, err := strconv.ParseInt(ctx.FormString("size"), 10, 64)
	if err != nil {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	item, err := softwarecenter.BeginUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.FormString("parent"), ctx.FormString("name"), ctx.FormString("client_key"), size)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, item)
}

func SoftwareUploadStatus(ctx *context.Context) {
	item, err := softwarecenter.GetUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, item)
}

func SoftwareUploadChunk(ctx *context.Context) {
	offset, err := strconv.ParseInt(ctx.FormString("offset"), 10, 64)
	if err != nil || offset < 0 {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	size, err := strconv.ParseInt(ctx.FormString("size"), 10, 64)
	if err != nil || size <= 0 || size > softwarecenter.ChunkSize || (ctx.Req.ContentLength >= 0 && ctx.Req.ContentLength != size) {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	body := http.MaxBytesReader(ctx.Resp, ctx.Req.Body, size+1)
	defer body.Close()
	item, err := softwarecenter.AppendUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id"), offset, size, ctx.Req.Header.Get("X-Chunk-SHA256"), body)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, item)
}

func SoftwareUploadComplete(ctx *context.Context) {
	item, err := softwarecenter.CompleteUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, item)
}

func SoftwareUploadCancel(ctx *context.Context) {
	if err := softwarecenter.CancelUpload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id")); err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func SoftwareDownloads(ctx *context.Context) {
	items, err := softwarecenter.Downloads(ctx, ctx.Org.Organization.ID, ctx.Doer.ID)
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, items)
}

func SoftwareDownloadBegin(ctx *context.Context) {
	if err := ctx.Req.ParseForm(); err != nil {
		softwareError(ctx, softwarecenter.ErrInvalid)
		return
	}
	job, err := softwarecenter.CreateDownload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.FormStrings("ids"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, job)
}

func SoftwareDownloadProgress(ctx *context.Context) {
	job, err := softwarecenter.GetDownload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	ctx.JSON(http.StatusOK, softwarecenter.DownloadProgress(job))
}

func SoftwareDownloadRemove(ctx *context.Context) {
	if err := softwarecenter.RemoveDownload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id")); err != nil {
		softwareError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

type downloadWriter struct {
	http.ResponseWriter
	update func(int64, string)
	status int
	bytes  int64
}

func (w *downloadWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *downloadWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	w.update(int64(n), "")
	return n, err
}

type writerOnly struct{ io.Writer }

// ReadFrom restores sendfile for single-blob downloads while keeping progress for virtual ZIPs.
func (w *downloadWriter) ReadFrom(source io.Reader) (int64, error) {
	inner := source
	if limited, ok := inner.(*io.LimitedReader); ok {
		inner = limited.R
	}
	if _, ok := inner.(*os.File); ok {
		if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
			n, err := readerFrom.ReadFrom(source)
			w.bytes += n
			w.update(n, "")
			return n, err
		}
	}
	return io.Copy(writerOnly{w}, source)
}

var softwareDownloadSlots = make(chan struct{}, 8)

func softwareDownloadReceiptRequired(transfer, state string) bool {
	return transfer != "managed" || state == "complete"
}

func SoftwareDownloadFile(ctx *context.Context) {
	select {
	case softwareDownloadSlots <- struct{}{}:
		defer func() { <-softwareDownloadSlots }()
	default:
		softwareError(ctx, softwarecenter.ErrBusy)
		return
	}
	job, file, err := softwarecenter.OpenDownload(ctx, ctx.Org.Organization.ID, ctx.Doer.ID, ctx.PathParam("id"))
	if err != nil {
		softwareError(ctx, err)
		return
	}
	defer file.Close()
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	if !job.Single {
		ctx.Resp.Header().Set("Content-Type", "application/zip")
	}
	ctx.Resp.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": job.Name}))
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	ctx.Resp.Header().Set("Cache-Control", "private, no-cache")
	ctx.Resp.Header().Set("ETag", job.ETag)
	offset := int64(0)
	if text, ok := strings.CutPrefix(ctx.Req.Header.Get("Range"), "bytes="); ok && !strings.Contains(text, ",") {
		start, end, _ := strings.Cut(text, "-")
		if start == "" {
			if suffix, e := strconv.ParseInt(end, 10, 64); e == nil && suffix > 0 {
				offset = max(0, job.Size-suffix)
			}
		}
		if n, e := strconv.ParseInt(start, 10, 64); e == nil && n >= 0 && n < job.Size {
			offset = n
		}
	}
	update := softwarecenter.StartDownloadProgress(job.ID, job.Size, offset)
	writer := &downloadWriter{ResponseWriter: ctx.Resp, update: update, status: http.StatusOK}
	http.ServeContent(writer, ctx.Req, job.Name, time.Unix(job.Created, 0), file)
	state := "paused"
	if writer.status >= 400 {
		state = "failed"
	} else if writer.status == http.StatusNotModified || offset+writer.bytes >= job.Size {
		state = "complete"
	}
	update(0, state)
	// Managed clients commit each range locally; avoid rewriting org metadata per chunk.
	if !softwareDownloadReceiptRequired(ctx.Req.Header.Get("X-GitSSS-Transfer"), state) {
		return
	}
	receiptContext, cancel := stdcontext.WithTimeout(stdcontext.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := softwarecenter.FinishDownload(receiptContext, ctx.Org.Organization.ID, ctx.Doer.ID, job.ID); err != nil {
		log.Warn("Software download receipt pending: %v", err)
	}
}
