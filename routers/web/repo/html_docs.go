// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	git_model "github.com/neko233-com/gitsss/models/git"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/lfs"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/util"
	"github.com/neko233-com/gitsss/services/context"
	repo_service "github.com/neko233-com/gitsss/services/repository"
)

const htmlDocsCSP = "default-src 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'self'; form-action 'none'; frame-ancestors 'self'; sandbox allow-scripts"

func resolveHTMLDocs(repo *git.Repository, settings *repo_service.HTMLDocsSettings, defaultRef string) (*git.Commit, string, error) {
	ref := settings.Ref
	if ref == "" {
		ref = defaultRef
	}
	commit, err := repo.GetCommit(ref)
	if err != nil {
		return nil, "", err
	}
	candidates := []string{path.Join(settings.Directory, settings.Entry)}
	if settings.Directory == "" {
		candidates = []string{"docs/site/" + settings.Entry, "docs/" + settings.Entry, "site/" + settings.Entry, "_site/" + settings.Entry, settings.Entry}
	}
	for _, candidate := range candidates {
		entry, err := commit.GetTreeEntryByPath(candidate)
		if err == nil && entry.IsRegular() {
			return commit, candidate, nil
		}
	}
	return commit, "", nil
}

// HTMLDocs keeps the repository workspace around an isolated, Git-backed HTML site.
func HTMLDocs(ctx *context.Context) {
	if !checkDownloadTokenScope(ctx) {
		return
	}
	ctx.Data["Title"] = ctx.Tr("repo.docs.title")
	ctx.Data["PageIsHTMLDocs"] = true
	ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
	settings, err := repo_service.GetHTMLDocsSettings(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetHTMLDocsSettings", err)
		return
	}
	ctx.Data["DocsSettings"] = settings
	ctx.Data["CanManageDocs"] = ctx.Repo.Permission.IsAdmin()
	if settings.Enabled && !ctx.Repo.Repository.IsEmpty {
		repo, err := git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer repo.Close()
		commit, entry, err := resolveHTMLDocs(repo, settings, ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			ctx.Data["DocsRefMissing"] = true
		} else if entry != "" {
			access := repo_service.HTMLDocsAccess{RepoID: ctx.Repo.Repository.ID, Revision: settings.Revision, Commit: commit.ID.String(), Expires: time.Now().Add(30 * time.Minute).Unix()}
			if ctx.Doer != nil {
				access.UserID = ctx.Doer.ID
			}
			token, err := repo_service.SignHTMLDocsAccess(access)
			if err != nil {
				ctx.ServerError("SignHTMLDocsAccess", err)
				return
			}
			ctx.Data["DocsEntry"] = entry
			ctx.Data["DocsCommit"] = commit.ID.String()
			ctx.Data["DocsContentURL"] = ctx.Repo.RepoLink + "/-/docs/content/" + commit.ID.String() + "/" + token + "/" + util.PathEscapeSegments(entry)
			ctx.Data["DocsSourceURL"] = ctx.Repo.RepoLink + "/src/commit/" + commit.ID.String() + "/" + util.PathEscapeSegments(entry)
		}
	}
	ctx.HTML(http.StatusOK, templates.TplName("repo/html_docs"))
}

// HTMLPreview pins an arbitrary repository HTML file and its relative assets to one commit.
func HTMLPreview(ctx *context.Context) {
	if !checkDownloadTokenScope(ctx) {
		return
	}
	file, err := repo_service.CleanHTMLDocsPath(ctx.PathParam("*"))
	if err != nil || (strings.ToLower(path.Ext(file)) != ".html" && strings.ToLower(path.Ext(file)) != ".htm") {
		ctx.NotFound(err)
		return
	}
	sha := ctx.PathParam("sha")
	if !htmlPreviewObjectID(sha) {
		ctx.NotFound(nil)
		return
	}
	repo, err := git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer repo.Close()
	commit, err := repo.GetCommit(sha)
	if err != nil {
		ctx.NotFound(err)
		return
	}
	entry, err := commit.GetTreeEntryByPath(file)
	if err != nil || !entry.IsRegular() {
		ctx.NotFound(err)
		return
	}
	settings, err := repo_service.GetHTMLDocsSettings(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetHTMLDocsSettings", err)
		return
	}
	access := repo_service.HTMLDocsAccess{RepoID: ctx.Repo.Repository.ID, Revision: settings.Revision, Commit: commit.ID.String(), Expires: time.Now().Add(30 * time.Minute).Unix(), Preview: true}
	if ctx.Doer != nil {
		access.UserID = ctx.Doer.ID
	}
	token, err := repo_service.SignHTMLDocsAccess(access)
	if err != nil {
		ctx.ServerError("SignHTMLDocsAccess", err)
		return
	}
	ctx.Data["Title"] = ctx.Tr("repo.html_preview")
	ctx.Data["DocsPreview"] = true
	ctx.Data["DocsSettings"] = settings
	ctx.Data["DocsEntry"] = file
	ctx.Data["DocsCommit"] = commit.ID.String()
	ctx.Data["DocsContentURL"] = ctx.Repo.RepoLink + "/-/docs/content/" + commit.ID.String() + "/" + token + "/" + util.PathEscapeSegments(file)
	ctx.Data["DocsSourceURL"] = ctx.Repo.RepoLink + "/src/commit/" + commit.ID.String() + "/" + util.PathEscapeSegments(file)
	ctx.HTML(http.StatusOK, templates.TplName("repo/html_docs"))
}

func htmlPreviewObjectID(sha string) bool {
	if len(sha) != 40 && len(sha) != 64 {
		return false
	}
	for _, char := range sha {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

// HTMLDocsAuthorize replaces unavailable SameSite cookies only on this read-only content route.
func HTMLDocsAuthorize(ctx *context.Context) {
	access, err := repo_service.VerifyHTMLDocsAccess(ctx.PathParam("access"), time.Now())
	if err != nil || access.Commit != ctx.PathParam("sha") {
		ctx.NotFound(nil)
		return
	}
	ctx.Doer = nil
	ctx.IsSigned = false
	if access.UserID > 0 {
		user, err := user_model.GetUserByID(ctx, access.UserID)
		if err != nil || !user.IsActive || user.ProhibitLogin || user.MustChangePassword {
			ctx.NotFound(nil)
			return
		}
		ctx.Doer = user
		ctx.IsSigned = true
	} else if setting.Service.RequireSignInViewStrict {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["HTMLDocsAccess"] = access
}

// HTMLDocsPost is restricted to repository administrators by the route middleware.
func HTMLDocsPost(ctx *context.Context) {
	settings := &repo_service.HTMLDocsSettings{
		Enabled: ctx.FormBool("enabled"), Ref: ctx.FormString("ref"),
		Directory: ctx.FormString("directory"), Entry: ctx.FormString("entry"),
		Revision: ctx.FormInt64("revision"),
	}
	if err := repo_service.SaveHTMLDocsSettings(ctx, ctx.Repo.Repository.ID, settings); err != nil {
		if errors.Is(err, repo_service.ErrHTMLDocsConflict) {
			ctx.HTTPError(http.StatusConflict, ctx.Locale.TrString("repo.docs.conflict"))
			return
		}
		ctx.Flash.Error(ctx.Tr("repo.docs.invalid", err))
	} else {
		ctx.Flash.Success(ctx.Tr("repo.docs.saved"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/-/docs")
}

// HTMLDocsContent never serves repository HTML with the application's origin privileges.
func HTMLDocsContent(ctx *context.Context) {
	if !checkDownloadTokenScope(ctx) {
		return
	}
	settings, err := repo_service.GetHTMLDocsSettings(ctx, ctx.Repo.Repository.ID)
	if err != nil || ctx.Repo.Repository.IsEmpty {
		ctx.NotFound(err)
		return
	}
	access, ok := ctx.Data["HTMLDocsAccess"].(*repo_service.HTMLDocsAccess)
	if !ok || access.RepoID != ctx.Repo.Repository.ID || access.Revision != settings.Revision {
		ctx.NotFound(nil)
		return
	}
	repo, err := git.OpenRepository(ctx, ctx.Repo.Repository.RepoPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer repo.Close()
	sha := ctx.PathParam("sha")
	if _, err := git.NewIDFromString(sha); err != nil {
		ctx.NotFound(err)
		return
	}
	var commit *git.Commit
	root := ""
	if access.Preview {
		commit, err = repo.GetCommit(sha)
	} else {
		if !settings.Enabled {
			ctx.NotFound(nil)
			return
		}
		pinned := *settings
		pinned.Ref = sha
		var siteEntry string
		commit, siteEntry, err = resolveHTMLDocs(repo, &pinned, ctx.Repo.Repository.DefaultBranch)
		if siteEntry == "" {
			ctx.NotFound(err)
			return
		}
		root = settings.Directory
		if root == "" {
			root = strings.TrimSuffix(strings.TrimSuffix(siteEntry, settings.Entry), "/")
		}
	}
	if err != nil {
		ctx.NotFound(err)
		return
	}
	file, err := repo_service.CleanHTMLDocsPath(ctx.PathParam("*"))
	if err != nil || (root != "" && file != root && !strings.HasPrefix(file, root+"/")) {
		ctx.NotFound(err)
		return
	}
	entry, err := commit.GetTreeEntryByPath(file)
	if err == nil && entry.IsDir() {
		if !strings.HasSuffix(ctx.Req.URL.Path, "/") {
			ctx.Redirect(ctx.Req.URL.Path + "/")
			return
		}
		file = path.Join(file, "index.html")
		entry, err = commit.GetTreeEntryByPath(file)
	}
	if err != nil || !entry.IsRegular() || entry.Blob().Size() > 32*1024*1024 {
		ctx.NotFound(err)
		return
	}
	typeName := mime.TypeByExtension(strings.ToLower(path.Ext(file)))
	if typeName == "" {
		typeName = "application/octet-stream"
	}
	header := ctx.Resp.Header()
	header.Set("Content-Type", typeName)
	// Explicit source paths work with the sandbox's opaque origin; 'self' alone does not.
	if strings.ContainsAny(ctx.Req.Host, " \t\r\n;,\"'") {
		ctx.NotFound(nil)
		return
	}
	scheme := "http"
	if ctx.Req.TLS != nil || strings.HasPrefix(setting.AppURL, "https://") {
		scheme = "https"
	}
	assetSource := scheme + "://" + ctx.Req.Host + ctx.Repo.RepoLink + "/-/docs/content/" + sha + "/" + ctx.PathParam("access") + "/"
	policy := htmlDocsCSP
	for _, directive := range []string{"img-src", "style-src", "script-src", "font-src", "base-uri"} {
		policy = strings.ReplaceAll(policy, directive+" 'self'", directive+" "+assetSource)
	}
	policy = strings.ReplaceAll(policy, "connect-src 'none'", "connect-src "+assetSource)
	header.Set("Content-Security-Policy", policy)
	header.Set("X-Content-Type-Options", "nosniff")
	// The scoped capability authorizes reads without cookies, including opaque-origin modules.
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
	var reader io.ReadCloser
	buffer, err := entry.Blob().GetBlobBytes(lfs.MetaFileMaxSize)
	if err != nil {
		ctx.ServerError("ReadHTMLDocs", err)
		return
	}
	pointer, _ := lfs.ReadPointerFromBuffer(buffer)
	if pointer.IsValid() {
		meta, lookupErr := git_model.GetLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, pointer.Oid)
		if lookupErr != nil || meta == nil || meta.Size > 32*1024*1024 {
			ctx.NotFound(lookupErr)
			return
		}
		reader, err = lfs.ReadMetaObject(meta.Pointer)
	} else {
		reader, err = entry.Blob().DataAsync()
	}
	if err != nil {
		ctx.ServerError("ReadHTMLDocs", err)
		return
	}
	defer reader.Close()
	ctx.Resp.WriteHeader(http.StatusOK)
	_, _ = io.Copy(ctx.Resp, reader)
}
