// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageRegistriesGoHTTPS(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://example.test/")()
	defer test.MockVariableValue(&setting.Packages.Enabled, true)()
	defer test.MockVariableValue(&setting.PackageMirror.GoClientBaseURL)()
	ctx, _ := contexttest.MockContext(t, "GET /")
	owner := &user_model.User{Name: "engineering", Type: user_model.UserTypeOrganization}
	setting.PackageMirror.GoClientBaseURL = ""
	preparePackageRegistries(ctx, owner)
	registries := ctx.Data["PackageRegistries"].([]packageRegistry)
	require.Equal(t, "Go Modules", registries[1].Name)
	assert.NotContains(t, registries[1].Command, "go env -w")
	setting.PackageMirror.GoClientBaseURL = "https://mirror.example.test:3443/git"
	preparePackageRegistries(ctx, owner)
	registries = ctx.Data["PackageRegistries"].([]packageRegistry)
	assert.Equal(t, "https://mirror.example.test:3443/git/api/packages/engineering/go", registries[1].URL)
	assert.Contains(t, registries[1].Command, registries[1].URL)
}
