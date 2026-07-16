// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"path"
	"time"

	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/httpcache"
	"github.com/neko233-com/gitsss/modules/httplib"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/services/context"
)

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Base, repo *repo_model.Repository, filePath string, blob *git.Blob, lastModified *time.Time) error {
	if httpcache.HandleGenericETagPrivateCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
		return nil
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return err
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer dataRc.Close()

	if lastModified == nil {
		lastModified = new(time.Time)
	}
	httplib.ServeUserContentByReader(ctx.Req, ctx.Resp, blob.Size(), dataRc, httplib.ServeHeaderOptions{
		Filename:      path.Base(filePath),
		CacheIsPublic: !repo.IsPrivate && repo.Owner.Visibility == structs.VisibleTypePublic,
		CacheDuration: setting.StaticCacheTime,
		LastModified:  *lastModified,
	})
	return nil
}
