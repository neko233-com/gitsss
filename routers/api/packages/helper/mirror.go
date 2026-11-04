// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helper

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	packages_model "github.com/neko233-com/gitsss/models/packages"
	"github.com/neko233-com/gitsss/modules/glob"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/packagemirror"
	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/util"
	"github.com/neko233-com/gitsss/services/context"

	"golang.org/x/mod/module"
)

var packageMirror = sync.OnceValues(func() (*packagemirror.Cache, error) {
	lfsRoot := ""
	if setting.LFS.Storage != nil {
		lfsRoot = setting.LFS.Storage.Path
	}
	directory, err := repolayout.ReserveExtension(setting.AppDataPath, setting.RepoRootPath, lfsRoot, "package-mirror")
	if err != nil {
		return nil, err
	}
	cfg := setting.PackageMirror
	return packagemirror.New(packagemirror.Config{
		Directory: directory, Secret: setting.SecretKey,
		Upstreams:   map[string]string{"maven": cfg.MavenURL, "go": cfg.GoURL, "npm": cfg.NpmURL, "pypi": cfg.PyPIURL, "cargo": cfg.CargoIndexURL, "cargo-file": cfg.CargoFilesURL, "nuget": cfg.NuGetURL, "composer": cfg.ComposerURL},
		FileHosts:   map[string][]string{"npm": cfg.NpmFileHosts, "pypi": cfg.PyPIFileHosts, "go": cfg.GoFileHosts, "composer": cfg.ComposerFileHosts},
		MetadataTTL: cfg.MetadataTTL, ArtifactTTL: cfg.ArtifactTTL,
		Timeout: cfg.Timeout, MaxSize: cfg.MaxSize, MaxFileSize: cfg.MaxFileSize, MaxEntries: cfg.MaxEntries,
	}), nil
})

func mirrorEligible(ctx *context.Context, kind, name string) (bool, error) {
	if !setting.PackageMirror.Enabled || name == "" {
		return false, nil
	}
	if strings.HasPrefix(strings.ToLower(name), "@"+ctx.Package.Owner.LowerName+"/") {
		return false, nil
	}
	appURL, _ := url.Parse(setting.AppURL)
	if kind == "go" && strings.HasPrefix(strings.ToLower(name), strings.ToLower(appURL.Hostname())+"/") {
		return false, nil
	}
	for _, pattern := range setting.PackageMirror.PrivatePatterns {
		matcher, err := glob.Compile(pattern)
		if err != nil {
			return false, err
		}
		if matcher.Match(name) {
			return false, nil
		}
	}
	names := []string{name}
	if kind == "maven" {
		names = append(names, strings.ReplaceAll(name, ":", "-"))
	}
	packageType := map[string]packages_model.Type{"maven": packages_model.TypeMaven, "go": packages_model.TypeGo, "npm": packages_model.TypeNpm, "pypi": packages_model.TypePyPI, "cargo": packages_model.TypeCargo, "nuget": packages_model.TypeNuGet, "composer": packages_model.TypeComposer}[kind]
	for _, candidate := range names {
		_, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packageType, candidate)
		if err == nil {
			return false, nil
		}
		if !errors.Is(err, util.ErrNotExist) {
			return false, err
		}
	}
	return true, nil
}

// TryServeCargoMirror is used only after an authorized local Cargo lookup misses.
func TryServeCargoMirror(ctx *context.Context, name, version string) bool {
	eligible, err := mirrorEligible(ctx, "cargo", name)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return true
	}
	if !eligible {
		return false
	}
	name = strings.ToLower(name)
	if name == "" || len(name) > 64 || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_-") != "" || strings.Trim(version, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.+-") != "" {
		ctx.Status(http.StatusBadRequest)
		return true
	}
	var relative string
	kind, metadata := "cargo", true
	switch len(name) {
	case 1:
		relative = "1/" + name
	case 2:
		relative = "2/" + name
	case 3:
		relative = "3/" + name[:1] + "/" + name
	default:
		relative = name[:2] + "/" + name[2:4] + "/" + name
	}
	if version != "" {
		kind, relative, metadata = "cargo-file", name+"/"+name+"-"+version+".crate", false
	}
	cache, err := packageMirror()
	if err != nil {
		ctx.Status(http.StatusServiceUnavailable)
		return true
	}
	entry, err := cache.Get(ctx, kind, relative, metadata)
	if err != nil {
		mirrorError(ctx, err)
		return true
	}
	serveMirrorEntry(ctx, entry)
	return true
}

// TryServeMirror is called only after an authorized local read returned 404.
func TryServeMirror(ctx *context.Context, kind, name string) bool {
	eligible, err := mirrorEligible(ctx, kind, name)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return true
	}
	if !eligible {
		return false
	}
	prefix := "/api/packages/" + ctx.PathParam("username") + "/" + kind + "/"
	relative, ok := strings.CutPrefix(ctx.Req.URL.Path, prefix)
	if !ok && setting.AppSubURL != "" {
		relative, ok = strings.CutPrefix(ctx.Req.URL.Path, setting.AppSubURL+prefix)
	}
	if !ok || relative == "" {
		return false
	}
	if kind == "nuget" {
		relative, ok = strings.CutPrefix(relative, "package/")
		if !ok || relative == "" {
			return false
		}
	}
	cache, err := packageMirror()
	if err != nil {
		ctx.Status(http.StatusServiceUnavailable)
		return true
	}
	metadata := (kind == "npm" && strings.TrimRight(relative, "/") == name) || kind == "pypi" || kind == "composer" || kind == "nuget" && strings.HasSuffix(relative, "/index.json") || strings.Contains(relative, "maven-metadata.xml") || strings.HasSuffix(relative, "/list") || strings.HasSuffix(relative, "/@latest")
	if kind == "pypi" {
		relative = name + "/"
	}
	entry, err := cache.Get(ctx, kind, relative, metadata)
	if err != nil {
		mirrorError(ctx, err)
		return true
	}
	registry := strings.TrimRight(setting.AppURL, "/") + "/api/packages/" + ctx.Package.Owner.Name + "/" + kind
	owner := strconv.FormatInt(ctx.Package.Owner.ID, 10)
	var body []byte
	switch {
	case kind == "npm" && metadata:
		body, err = cache.NpmMetadata(entry, owner, name, registry)
	case kind == "pypi":
		body, err = cache.PyPIMetadata(entry, owner, name, registry, relative)
	case kind == "composer":
		body, err = cache.ComposerMetadata(entry, owner, name, registry)
	}
	if err != nil {
		_ = os.Remove(entry.Path)
		ctx.Status(http.StatusBadGateway)
		return true
	}
	ctx.Resp.Header().Set("X-GitSSS-Mirror", entry.State)
	ctx.Resp.Header().Set("Cache-Control", "private, max-age=0")
	if body != nil {
		contentType := "application/json"
		if kind == "pypi" {
			contentType = "text/html; charset=utf-8"
		}
		ctx.Resp.Header().Set("Content-Type", contentType)
		http.ServeContent(ctx.Resp, ctx.Req, "", entry.Saved, bytes.NewReader(body))
	} else {
		serveMirrorEntry(ctx, entry)
	}
	return true
}

// MirrorGoSumDB preserves Go's signed public checksum database protocol.
func MirrorGoSumDB(ctx *context.Context) {
	if !setting.PackageMirror.Enabled {
		ctx.Status(http.StatusNotFound)
		return
	}
	relative := ctx.PathParam("*")
	if relative != "supported" && relative != "latest" && !strings.HasPrefix(relative, "lookup/") && !strings.HasPrefix(relative, "tile/") {
		ctx.Status(http.StatusNotFound)
		return
	}
	if lookup, ok := strings.CutPrefix(relative, "lookup/"); ok {
		modulePath, _, valid := strings.Cut(lookup, "@")
		name, err := module.UnescapePath(modulePath)
		if err != nil || !valid {
			ctx.Status(http.StatusBadRequest)
			return
		}
		eligible, err := mirrorEligible(ctx, "go", name)
		if err != nil || !eligible {
			ctx.Status(http.StatusNotFound)
			return
		}
	}
	cache, err := packageMirror()
	if err != nil {
		ctx.Status(http.StatusServiceUnavailable)
		return
	}
	entry, err := cache.Get(ctx, "go", "sumdb/sum.golang.org/"+relative, relative == "latest")
	if err != nil {
		mirrorError(ctx, err)
		return
	}
	serveMirrorEntry(ctx, entry)
}

func mirrorError(ctx *context.Context, err error) {
	log.Warn("Package mirror read failed: %v", err)
	if limited, ok := errors.AsType[*packagemirror.RateLimitError](err); ok && limited.RetryAfter != "" {
		ctx.Resp.Header().Set("Retry-After", limited.RetryAfter)
	}
	if upstream, ok := errors.AsType[*packagemirror.Error](err); ok {
		ctx.Status(upstream.Status)
	} else {
		ctx.Status(http.StatusBadGateway)
	}
}

func serveMirrorEntry(ctx *context.Context, entry *packagemirror.Entry) {
	f, reader, err := entry.Open()
	if err != nil {
		ctx.Status(http.StatusServiceUnavailable)
		return
	}
	defer f.Close()
	ctx.Resp.Header().Set("Content-Type", entry.ContentType)
	ctx.Resp.Header().Set("ETag", entry.ETag)
	ctx.Resp.Header().Set("X-GitSSS-Mirror", entry.State)
	ctx.Resp.Header().Set("Cache-Control", "private, max-age=0")
	http.ServeContent(ctx.Resp, ctx.Req, "", entry.Saved, reader)
}

// MirrorArtifact serves only signed metadata links and retains owner read authorization.
func MirrorArtifact(kind string) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if !setting.PackageMirror.Enabled {
			ctx.Status(http.StatusNotFound)
			return
		}
		cache, err := packageMirror()
		if err != nil {
			ctx.Status(http.StatusServiceUnavailable)
			return
		}
		artifact, err := cache.ResolveArtifact(kind, strconv.FormatInt(ctx.Package.Owner.ID, 10), ctx.PathParam("token"))
		if err != nil {
			mirrorError(ctx, err)
			return
		}
		eligible, err := mirrorEligible(ctx, kind, artifact.Package)
		if err != nil || !eligible {
			ctx.Status(http.StatusNotFound)
			return
		}
		metadata := false
		u, _ := url.Parse(artifact.URL)
		filename := path.Base(u.Path)
		if kind == "pypi" && ctx.PathParam("filename") == filename+".metadata" {
			u.Path += ".metadata"
			artifact.URL = u.String()
			metadata = true
		} else if ctx.PathParam("filename") != filename {
			ctx.Status(http.StatusBadRequest)
			return
		}
		entry, err := cache.GetURL(ctx, kind, artifact.URL, metadata)
		if err != nil {
			mirrorError(ctx, err)
			return
		}
		serveMirrorEntry(ctx, entry)
	}
}
