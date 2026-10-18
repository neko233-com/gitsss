// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	issues_model "github.com/neko233-com/gitsss/models/issues"
	"github.com/neko233-com/gitsss/models/organization"
	access_model "github.com/neko233-com/gitsss/models/perm/access"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	issue_indexer "github.com/neko233-com/gitsss/modules/indexer/issues"
	"github.com/neko233-com/gitsss/modules/optional"
	"github.com/neko233-com/gitsss/services/context"
)

type workspaceSearchRow struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Kind  string `json:"kind"`
}

// WorkspaceSearch uses browser-session authorization without exposing an API token to scripts.
func WorkspaceSearch(ctx *context.Context) {
	ctx.Resp.Header().Set("Cache-Control", "private, no-store")
	query := ctx.FormTrim("q")
	if len([]rune(query)) > 160 {
		ctx.HTTPError(http.StatusBadRequest)
		return
	}
	rows := []workspaceSearchRow{}
	if query == "" {
		ctx.JSON(http.StatusOK, rows)
		return
	}
	opts := repo_model.SearchRepoOptions{Actor: ctx.Doer, Keyword: query, Private: true, AllPublic: true, AllLimited: true, OrderBy: db.SearchOrderByAlphabetically}
	opts.Page, opts.PageSize = 1, 8
	repos, _, err := repo_model.SearchRepository(ctx, opts)
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	for _, repo := range repos {
		rows = append(rows, workspaceSearchRow{Title: repo.FullName(), URL: repo.Link(), Kind: "repos"})
	}
	opts.Keyword = ""
	opts.Page, opts.PageSize = 0, 0
	ids, _, err := repo_model.SearchRepositoryIDs(ctx, opts)
	if err != nil {
		ctx.ServerError("SearchRepositoryIDs", err)
		return
	}
	if len(ids) > 0 {
		options := &issue_indexer.SearchOptions{Keyword: query, RepoIDs: ids, IsClosed: optional.None[bool](), SortBy: issue_indexer.SortByCreatedDesc}
		options.Paginator = &db.ListOptions{Page: 1, PageSize: 8}
		issueIDs, _, err := issue_indexer.SearchIssues(ctx, options)
		if err != nil {
			ctx.ServerError("SearchIssues", err)
			return
		}
		issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs, true)
		if err != nil {
			ctx.ServerError("GetIssuesByIDs", err)
			return
		}
		for _, issue := range issues {
			if err := issue.LoadRepo(ctx); err != nil {
				continue
			}
			permission, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, ctx.Doer)
			if err == nil && permission.CanReadIssuesOrPulls(issue.IsPull) {
				rows = append(rows, workspaceSearchRow{Title: issue.Title, URL: issue.Link(), Kind: "issues"})
			}
		}
	}
	orgs, err := organization.GetUserOrgsList(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserOrgsList", err)
		return
	}
	for index, org := range orgs {
		if index >= 50 {
			break
		}
		if strings.Contains(strings.ToLower(org.Name+" "+org.FullName), strings.ToLower(query)) {
			rows = append(rows, workspaceSearchRow{Title: strings.TrimSpace(org.Name + " " + org.FullName), URL: org.OrganisationLink() + "/dashboard", Kind: "orgs"})
		}
	}
	ctx.JSON(http.StatusOK, rows)
}
