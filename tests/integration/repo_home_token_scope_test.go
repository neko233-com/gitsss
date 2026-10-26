// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/routers"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryAccessDeniedFeedback(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const repoPath = "/user2/repo2"

	MakeRequest(t, NewRequest(t, "GET", repoPath), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2"), http.StatusNotFound)

	session := loginUser(t, "user4")
	resp := session.MakeRequest(t, NewRequest(t, "GET", repoPath), http.StatusForbidden)
	assert.Contains(t, resp.Body.String(), "user2/repo2")

	token := getUserToken(t, "user4", auth_model.AccessTokenScopeReadRepository)

	apiReq := NewRequest(t, "GET", "/api/v1/repos/user2/repo2")
	apiReq.SetBasicAuth("user4", token)
	apiResp := MakeRequest(t, apiReq, http.StatusForbidden)
	assert.Contains(t, apiResp.Body.String(), "user2/repo2")

	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	publicOnlyReq := NewRequest(t, "GET", "/api/v1/repos/user2/repo2").AddTokenAuth(publicOnlyToken)
	MakeRequest(t, publicOnlyReq, http.StatusNotFound)

	gitReq := NewRequest(t, "GET", repoPath+".git/info/refs?service=git-upload-pack")
	gitReq.SetBasicAuth("user4", token)
	gitResp := MakeRequest(t, gitReq, http.StatusForbidden)
	assert.Contains(t, gitResp.Body.String(), "user2/repo2")
}

func TestRepositoryAccessDeniedWithRequiredSignIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	for _, repoPath := range []string{"/user2/repo2", "/user2/repo1", "/user2/missing-repository"} {
		resp := MakeRequest(t, NewRequest(t, "GET", repoPath), http.StatusNotFound)
		assert.Empty(t, resp.Header().Get("Location"))
		htmlReq := NewRequest(t, "GET", repoPath)
		htmlReq.Header.Set("Accept", "text/html")
		htmlResp := MakeRequest(t, htmlReq, http.StatusNotFound)
		assert.Contains(t, htmlResp.Body.String(), "Your browser is not signed in.")
		doc := NewHTMLParser(t, htmlResp.Body)
		signInURL, exists := doc.doc.Find(".status-page-error a.ui.primary.button").Attr("href")
		assert.True(t, exists)
		assert.Contains(t, signInURL, "/user/login?redirect_to=")
		assert.Empty(t, htmlResp.Header().Get("Location"))
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos"+repoPath), http.StatusNotFound)
	}
	MakeRequest(t, NewRequest(t, "GET", "/explore/repos"), http.StatusSeeOther)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/version"), http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo2.git/info/refs?service=git-upload-pack"), http.StatusUnauthorized)

	session := loginUser(t, "user4")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo2"), http.StatusForbidden)
	assert.Contains(t, resp.Body.String(), "user2/repo2")
	token := getUserToken(t, "user4", auth_model.AccessTokenScopeReadRepository)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2").AddTokenAuth(token), http.StatusForbidden)
}

// TestRepoHomeContentTokenScopes ensures the web repository home page enforces the
// repository read scope (and public-only confinement) of an API token used via basic
// auth, so a wrongly-scoped token cannot read private repository content.
func TestRepoHomeContentTokenScopes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		requireSignIn bool
	}{
		{name: "optional-signin"},
		{name: "required-signin", requireSignIn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()
			defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, tc.requireSignIn)()
			defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()
			testRepoHomeContentTokenScopes(t)
		})
	}
}

func testRepoHomeContentTokenScopes(t *testing.T) {
	t.Helper()

	// user2/repo2 is a private repository owned by user2
	const url = "/user2/repo2"

	// a token without repository scope must be denied
	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)
	reqDenied := NewRequest(t, "GET", url)
	reqDenied.SetBasicAuth("user2", miscToken)
	MakeRequest(t, reqDenied, http.StatusForbidden)

	// a public-only token must be denied on a private repo
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", url)
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	MakeRequest(t, reqPublicOnly, http.StatusForbidden)

	// a token with repository read scope is allowed
	ownerReadToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	reqAllowed := NewRequest(t, "GET", url)
	reqAllowed.SetBasicAuth("user2", ownerReadToken)
	MakeRequest(t, reqAllowed, http.StatusOK)
}
