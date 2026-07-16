// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/git"
	code_indexer "github.com/neko233-com/gitsss/modules/indexer/code"
	"github.com/neko233-com/gitsss/modules/indexer/code/gitgrep"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/routers/common"
	"github.com/neko233-com/gitsss/services/context"
)

const tplSearch templates.TplName = "repo/search"

// Search render repository search page
func Search(ctx *context.Context) {
	ctx.Data["PageIsViewCode"] = true
	prepareSearch := common.PrepareCodeSearch(ctx)
	if prepareSearch.Keyword == "" {
		ctx.HTML(http.StatusOK, tplSearch)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var total int64
	var searchResults []*code_indexer.Result
	var searchResultLanguages []*code_indexer.SearchResultLanguages
	if setting.Indexer.RepoIndexerEnabled {
		var err error
		total, searchResults, searchResultLanguages, err = code_indexer.PerformSearch(ctx, &code_indexer.SearchOptions{
			RepoIDs:    []int64{ctx.Repo.Repository.ID},
			Keyword:    prepareSearch.Keyword,
			SearchMode: prepareSearch.SearchMode,
			Language:   prepareSearch.Language,
			Paginator: &db.ListOptions{
				Page:     page,
				PageSize: setting.UI.RepoSearchPagingNum,
			},
		})
		if err != nil {
			if code_indexer.IsAvailable(ctx) {
				ctx.ServerError("SearchResults", err)
				return
			}
			ctx.Data["CodeIndexerUnavailable"] = true
		} else {
			ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable(ctx)
		}
	} else {
		var err error
		// ref should be default branch or the first existing branch
		searchRef := git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch)
		searchResults, total, err = gitgrep.PerformSearch(ctx, page, ctx.Repo.Repository.ID, ctx.Repo.GitRepo, searchRef, prepareSearch.Keyword, prepareSearch.SearchMode)
		if err != nil {
			ctx.ServerError("gitgrep.PerformSearch", err)
			return
		}
	}

	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["SearchResults"] = searchResults
	ctx.Data["SearchResultLanguages"] = searchResultLanguages

	pager := context.NewPagination(total, setting.UI.RepoSearchPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSearch)
}
