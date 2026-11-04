// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/neko233-com/gitsss/modules/homelinks"
	"github.com/neko233-com/gitsss/modules/setting"
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
	view := homelinks.FindPage(shared.Links, personal.Links, ctx.FormString("links_query"), ctx.FormInt("links_page"))
	returnPath := homeLinksReturnPath(ctx)
	ctx.Data["HomeLinksPage"] = view
	ctx.Data["HomeLinksPath"] = returnPath
	ctx.Data["SharedHomeLinksRevision"] = shared.LoadedRevision()
	ctx.Data["PersonalHomeLinksRevision"] = personal.LoadedRevision()
	if view.Number > 1 {
		ctx.Data["HomeLinksPreviousURL"] = homeLinksPageURL(returnPath, view.Number-1, view.Query)
	}
	if view.Number < view.Pages {
		ctx.Data["HomeLinksNextURL"] = homeLinksPageURL(returnPath, view.Number+1, view.Query)
	}
	return true
}

func homeLinksReturnPath(ctx *context.Context) string {
	requestPath := ctx.Req.URL.Path
	if ctx.Req.Method == http.MethodPost {
		requestPath = ctx.FormString("return_path")
	} else if setting.AppSubURL != "" && !strings.HasPrefix(requestPath, setting.AppSubURL+"/") {
		requestPath = setting.AppSubURL + requestPath
	}
	if !strings.HasPrefix(requestPath, setting.AppSubURL+"/") || strings.HasPrefix(requestPath, "//") || strings.ContainsAny(requestPath, "?#\\\r\n") || path.Clean(requestPath) != requestPath {
		return setting.AppSubURL + "/"
	}
	relative := strings.TrimPrefix(requestPath, setting.AppSubURL)
	parts := strings.Split(strings.Trim(relative, "/"), "/")
	if relative == "/" || len(parts) == 3 && parts[0] == "org" && parts[1] != "" && parts[2] == "dashboard" {
		return requestPath
	}
	return setting.AppSubURL + "/"
}

func homeLinksPageURL(returnPath string, page int, query string) string {
	params := url.Values{"tab": {"links"}, "links_page": {strconv.Itoa(page)}}
	if query != "" {
		params.Set("links_query", query)
	}
	return returnPath + "?" + params.Encode()
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
	var err error
	switch ctx.FormString("operation") {
	case "":
		err = homelinks.Save(ctx, ownerID, ctx.FormString("revision"), links)
	case "page":
		indexes := ctx.FormStrings("item_index")
		originalStrings := ctx.FormStrings("page_index")
		if len(indexes) != len(links) || len(indexes) > homelinks.MaxPageRows || len(originalStrings) > homelinks.PageSize {
			ctx.Status(http.StatusBadRequest)
			return
		}
		original := make([]int, len(originalStrings))
		submitted := make([]homelinks.IndexedLink, 0, len(indexes))
		for i, raw := range originalStrings {
			original[i], err = strconv.Atoi(raw)
			if err != nil {
				ctx.Status(http.StatusBadRequest)
				return
			}
		}
		deletedIndexes := ctx.FormStrings("delete_index")
		if len(deletedIndexes) > homelinks.PageSize {
			ctx.Status(http.StatusBadRequest)
			return
		}
		deleted := make(map[int]bool, len(deletedIndexes))
		for _, raw := range deletedIndexes {
			index, parseErr := strconv.Atoi(raw)
			if parseErr != nil || deleted[index] || !slices.Contains(original, index) {
				ctx.Status(http.StatusBadRequest)
				return
			}
			deleted[index] = true
		}
		for i, raw := range indexes {
			index, parseErr := strconv.Atoi(raw)
			if parseErr != nil {
				ctx.Status(http.StatusBadRequest)
				return
			}
			if index == -1 && strings.TrimSpace(links[i].Title) == "" && strings.TrimSpace(links[i].URL) == "" {
				continue
			}
			if deleted[index] {
				continue
			}
			submitted = append(submitted, homelinks.IndexedLink{Index: index, Link: links[i]})
		}
		err = homelinks.ApplyPage(ctx, ownerID, ctx.FormString("revision"), original, submitted)
	default:
		ctx.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
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
	if ctx.FormString("operation") == "page" && ctx.Req.Header.Get("X-GitSSS-Partial") != "1" {
		ctx.Redirect(homeLinksPageURL(homeLinksReturnPath(ctx), max(ctx.FormInt("links_page"), 1), ctx.FormString("links_query")), http.StatusSeeOther)
		return
	}
	if prepareHomeLinks(ctx) {
		ctx.HTML(http.StatusOK, "user/dashboard/home_links")
	}
}
