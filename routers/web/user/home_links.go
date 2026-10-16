// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"

	"github.com/neko233-com/gitsss/modules/homelinks"
	"github.com/neko233-com/gitsss/services/context"
)

func prepareHomeLinks(ctx *context.Context) bool {
	shared, err := homelinks.Load(ctx, 0)
	if err != nil {
		ctx.ServerError("LoadSharedHomeLinks", err)
		return false
	}
	personal, err := homelinks.Load(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("LoadPersonalHomeLinks", err)
		return false
	}
	ctx.Data["SharedHomeLinks"], ctx.Data["PersonalHomeLinks"] = shared, personal
	return true
}

// HomeLinksPost saves only the current user's links or administrator-managed shared links.
func HomeLinksPost(ctx *context.Context) {
	if !ctx.IsSigned || ctx.Doer == nil {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	ownerID := ctx.Doer.ID
	switch ctx.FormString("scope") {
	case "shared":
		if !ctx.Doer.IsAdmin {
			ctx.Status(http.StatusForbidden)
			return
		}
		ownerID = 0
	case "personal":
	default:
		ctx.Status(http.StatusBadRequest)
		return
	}
	titles, urls := ctx.FormStrings("title"), ctx.FormStrings("url")
	if len(titles) != len(urls) {
		ctx.Status(http.StatusBadRequest)
		return
	}
	links := make([]homelinks.Link, len(titles))
	for i := range titles {
		links[i] = homelinks.Link{Title: titles[i], URL: urls[i]}
	}
	if err := homelinks.Save(ctx, ownerID, ctx.FormString("revision"), links); err != nil {
		switch {
		case errors.Is(err, homelinks.ErrConflict):
			ctx.Status(http.StatusConflict)
		case errors.Is(err, homelinks.ErrInvalid):
			ctx.Status(http.StatusBadRequest)
		default:
			ctx.ServerError("SaveHomeLinks", err)
		}
		return
	}
	if prepareHomeLinks(ctx) {
		ctx.HTML(http.StatusOK, "user/dashboard/home_links")
	}
}
