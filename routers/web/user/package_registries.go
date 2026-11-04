// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/url"
	"strings"

	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
)

type packageRegistry struct {
	Name, URL, Upstream, Command string
}

func preparePackageRegistries(ctx *context.Context, owner *user_model.User) {
	if !setting.Packages.Enabled || !owner.IsOrganization() {
		return
	}
	base := strings.TrimRight(setting.AppURL, "/") + "/api/packages/" + owner.Name
	pip := "python -m pip config set global.index-url " + base + "/pypi/simple/"
	appURL, _ := url.Parse(setting.AppURL)
	goBase := base
	if setting.PackageMirror.GoClientBaseURL != "" {
		goBase = strings.TrimRight(setting.PackageMirror.GoClientBaseURL, "/") + "/api/packages/" + owner.Name
	}
	goCommand := "go env -w GOPROXY=" + goBase + "/go"
	if appURL.Scheme == "http" && setting.PackageMirror.GoClientBaseURL == "" {
		goCommand = string(ctx.Tr("packages.mirror.go_https_required"))
	}
	if appURL.Scheme == "http" {
		pip += "\npython -m pip config set global.trusted-host " + appURL.Hostname()
	}
	ctx.Data["PackageRegistries"] = []packageRegistry{
		{Name: "Maven", URL: base + "/maven", Upstream: setting.PackageMirror.MavenURL, Command: "<mirrors>\n  <mirror>\n    <id>gitsss</id>\n    <mirrorOf>central</mirrorOf>\n    <url>" + base + "/maven</url>\n  </mirror>\n</mirrors>"},
		{Name: "Go Modules", URL: goBase + "/go", Upstream: setting.PackageMirror.GoURL, Command: goCommand},
		{Name: "PyPI / pip", URL: base + "/pypi/simple/", Upstream: setting.PackageMirror.PyPIURL, Command: pip},
		{Name: "npm", URL: base + "/npm/", Upstream: setting.PackageMirror.NpmURL, Command: "npm config set registry " + base + "/npm/"},
		{Name: "Cargo", URL: base + "/cargo/", Upstream: setting.PackageMirror.CargoIndexURL, Command: "[source.crates-io]\nreplace-with = \"gitsss\"\n\n[source.gitsss]\nregistry = \"sparse+" + base + "/cargo/\"\n\n[registries.gitsss]\nindex = \"sparse+" + base + "/cargo/\"\n\n[registry]\nglobal-credential-providers = [\"cargo:token\"]"},
		{Name: "NuGet", URL: base + "/nuget/index.json", Upstream: setting.PackageMirror.NuGetURL, Command: "dotnet nuget add source " + base + "/nuget/index.json --name gitsss"},
		{Name: "Composer", URL: base + "/composer", Upstream: setting.PackageMirror.ComposerURL, Command: "composer config repositories.gitsss composer " + base + "/composer\ncomposer config repositories.packagist.org false"},
	}
	ctx.Data["PackageMirrorEnabled"] = setting.PackageMirror.Enabled
}
