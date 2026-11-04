// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	packages_model "github.com/neko233-com/gitsss/models/packages"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/require"
)

func TestLanguagePackageRegistriesUnavailable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	paths := []string{
		"/api/packages/user2/go/example.com/example/@v/list",
		"/api/packages/user2/npm/example",
		"/api/packages/user2/pypi/simple/example",
		"/api/packages/user2/maven/org/example/example/1.0/example-1.0.jar",
		"/api/packages/user2/cargo/config.json",
		"/api/packages/user2/nuget/index.json",
		"/api/packages/user2/composer/packages.json",
	}
	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPost, http.MethodDelete} {
			MakeRequest(t, NewRequest(t, method, path).AddBasicAuth("user2"), http.StatusNotFound)
		}
	}
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWritePackage)
	for _, kind := range []packages_model.Type{packages_model.TypeGo, packages_model.TypeNpm, packages_model.TypePyPI, packages_model.TypeMaven, packages_model.TypeCargo, packages_model.TypeNuGet, packages_model.TypeComposer} {
		pkg, err := packages_model.TryInsertPackage(t.Context(), &packages_model.Package{OwnerID: 2, Type: kind, Name: "retired-example", LowerName: "retired-example"})
		require.NoError(t, err)
		_, err = packages_model.GetOrInsertVersion(t.Context(), &packages_model.PackageVersion{PackageID: pkg.ID, CreatorID: 2, Version: "1.0.0", LowerVersion: "1.0.0", MetadataJSON: "{}"})
		require.NoError(t, err)
		resource := "/api/v1/packages/user2/" + string(kind) + "/retired-example/1.0.0"
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			MakeRequest(t, NewRequest(t, method, resource).AddTokenAuth(token), http.StatusNotFound)
		}
		_, err = packages_model.GetVersionByNameAndVersion(t.Context(), 2, kind, "retired-example", "1.0.0")
		require.NoError(t, err)
	}
}
