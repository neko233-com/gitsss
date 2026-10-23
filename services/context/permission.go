// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"net/http"
	"slices"
	"strings"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unit"
	"github.com/neko233-com/gitsss/modules/setting"
)

// isOwnerHidden reports whether repo's owner is not publicly visible (a limited or private owner), so
// the owner's repositories must be hidden from callers that may only reach genuinely public resources.
func isOwnerHidden(ctx context.Context, repo *repo_model.Repository) bool {
	if err := repo.LoadOwner(ctx); err != nil || repo.Owner == nil {
		return true // fail closed if the owner visibility can't be determined
	}
	return !repo.Owner.Visibility.IsPublic()
}

// publicOnlyTokenDeniedRepo reports whether a public-only API token must be denied access to
// repo. A public-only token may only reach genuinely public resources, so it is denied for
// private repos and for repos owned by a non-public (limited or private) owner.
func publicOnlyTokenDeniedRepo(ctx context.Context, repo *repo_model.Repository) bool {
	if repo == nil {
		return false
	}
	return repo.IsPrivate || isOwnerHidden(ctx, repo)
}

// TokenIsPublicOnly reports whether the request is authenticated by a public-only API token. A
// non-token request, or a token with no recorded scope, is not public-only.
func TokenIsPublicOnly(ctx *Context) bool {
	if ctx.Data["IsApiToken"] != true {
		return false
	}
	scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if !ok {
		return false
	}
	publicOnly, _ := scope.PublicOnly()
	return publicOnly
}

// CheckTokenScopes checks whether the authenticated API token contains any of the given scopes.
func CheckTokenScopes(ctx *Context, repo *repo_model.Repository, scopes ...auth_model.AccessTokenScope) {
	if ctx.Data["IsApiToken"] != true {
		return
	}

	scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if !ok {
		return
	}

	publicOnly, err := scope.PublicOnly()
	if err != nil {
		ctx.ServerError("PublicOnly", err)
		return
	}

	if publicOnly && publicOnlyTokenDeniedRepo(ctx, repo) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	scopeMatched, err := scope.HasAnyScope(scopes...)
	if err != nil {
		ctx.ServerError("HasAnyScope", err)
		return
	}

	if !scopeMatched {
		ctx.HTTPError(http.StatusForbidden)
	}
}

// RepoAccessDenied gives signed-in users actionable feedback while keeping
// inaccessible repositories hidden from anonymous and public-only callers.
func (ctx *Context) RepoAccessDenied(repo *repo_model.Repository) {
	if !ctx.IsSigned || TokenIsPublicOnly(ctx) {
		ctx.NotFound(nil)
		return
	}

	ctx.Data["Title"] = ctx.Locale.TrString("repo.access_denied.title")
	ctx.Data["PageTitleCommon"] = setting.AppName
	ctx.Data["RepoFullName"] = repo.FullName()

	if strings.Contains(ctx.Req.Header.Get("Accept"), "text/html") {
		ctx.HTML(http.StatusForbidden, "status/403")
		return
	}

	message := strings.Join([]string{
		ctx.Locale.TrString("repo.access_denied.message"),
		ctx.Locale.TrString("repo.access_denied.repository") + ": " + repo.FullName(),
		ctx.Locale.TrString("repo.access_denied.next_step"),
	}, "\n")
	ctx.PlainText(http.StatusForbidden, message)
}

// RequireRepoAdmin returns a middleware for requiring repository admin permission
func RequireRepoAdmin() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.IsSigned || !ctx.Repo.Permission.IsAdmin() {
			ctx.RepoAccessDenied(ctx.Repo.Repository)
			return
		}
	}
}

// CanWriteToBranch checks if the user is allowed to write to the branch of the repo
func CanWriteToBranch() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName) {
			ctx.RepoAccessDenied(ctx.Repo.Repository)
			return
		}
	}
}

// RequireUnitWriter returns a middleware for requiring repository write to one of the unit permission
func RequireUnitWriter(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		if slices.ContainsFunc(unitTypes, ctx.Repo.Permission.CanWrite) {
			return
		}
		ctx.RepoAccessDenied(ctx.Repo.Repository)
	}
}

// RequireUnitReader returns a middleware for requiring repository write to one of the unit permission
func RequireUnitReader(unitTypes ...unit.Type) func(ctx *Context) {
	return func(ctx *Context) {
		for _, unitType := range unitTypes {
			if ctx.Repo.Permission.CanRead(unitType) {
				return
			}
			if unitType == unit.TypeCode && canWriteAsMaintainer(ctx) {
				return
			}
		}
		ctx.RepoAccessDenied(ctx.Repo.Repository)
	}
}

// CheckRepoScopedToken checks whether the authenticated API token has repo scope.
func CheckRepoScopedToken(ctx *Context, repo *repo_model.Repository, level auth_model.AccessTokenScopeLevel) {
	CheckTokenScopes(ctx, repo, auth_model.GetRequiredScopes(level, auth_model.AccessTokenScopeCategoryRepository)...)
}
