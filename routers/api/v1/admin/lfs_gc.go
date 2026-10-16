// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/lfsgc"
	"github.com/neko233-com/gitsss/services/context"
	repo_service "github.com/neko233-com/gitsss/services/repository"
)

// PostLFSGC scans or deletes unreferenced LFS objects for a site administrator.
func PostLFSGC(ctx *context.APIContext) {
	var body struct {
		Apply          bool   `json:"apply"`
		OlderThanHours int    `json:"older_than_hours"`
		Repository     string `json:"repository"`
		Force          bool   `json:"force"`
	}
	if ctx.Req.ContentLength != 0 {
		if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			ctx.APIError(http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	hours := body.OlderThanHours
	olderThan := time.Time{}
	if hours > 0 {
		olderThan = time.Now().Add(-time.Duration(hours) * time.Hour)
	}
	report, err := repo_service.CollectLFSGarbage(ctx, lfsgc.Request{
		Apply:      body.Apply,
		OlderThan:  olderThan,
		Repository: body.Repository,
		Force:      body.Force,
	})
	if err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]any{"error": err.Error(), "report": report})
		return
	}
	ctx.JSON(http.StatusOK, report)
}
