// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"github.com/neko233-com/gitsss/tests"
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
}
