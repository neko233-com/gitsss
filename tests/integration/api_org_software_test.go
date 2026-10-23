// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/services/softwarecenter"
	"github.com/neko233-com/gitsss/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIOrgSoftware(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	base := "/api/v1/orgs/org3/software"
	readToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization)
	writeToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization)
	otherToken := getUserToken(t, "user5", auth_model.AccessTokenScopeWriteOrganization)
	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopePublicOnly)
	MakeRequest(t, NewRequest(t, "GET", base), http.StatusUnauthorized)
	MakeRequest(t, NewRequest(t, "GET", base).AddTokenAuth(otherToken), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", base).AddTokenAuth(publicToken), http.StatusForbidden)
	MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/organize", &softwarecenter.OrganizeOptions{}).AddTokenAuth(readToken), http.StatusForbidden)
	MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/organize", &softwarecenter.OrganizeOptions{}).AddTokenAuth(otherToken), http.StatusNotFound)
	_, err := softwarecenter.CreateFolder(t.Context(), 3, 2, "", "SDK")
	require.NoError(t, err)
	resp := MakeRequest(t, NewRequest(t, "GET", base+"?recursive=true").AddTokenAuth(readToken), http.StatusOK)
	listing := DecodeJSON(t, resp, &softwarecenter.Listing{})
	assert.Equal(t, 1, listing.Total)
	resp = MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/organize", &softwarecenter.OrganizeOptions{}).AddTokenAuth(writeToken), http.StatusOK)
	preview := DecodeJSON(t, resp, &softwarecenter.OrganizeReport{})
	assert.False(t, preview.Applied)
	assert.Len(t, preview.Created, 5)
	assert.Len(t, preview.Moves, 1)
	resp = MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/organize", &softwarecenter.OrganizeOptions{Apply: true, Revision: preview.Revision}).AddTokenAuth(writeToken), http.StatusOK)
	assert.True(t, DecodeJSON(t, resp, &softwarecenter.OrganizeReport{}).Applied)
	MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/organize", &softwarecenter.OrganizeOptions{Apply: true, Revision: preview.Revision}).AddTokenAuth(writeToken), http.StatusConflict)
	resp = MakeRequest(t, NewRequest(t, "GET", base).AddTokenAuth(readToken), http.StatusOK)
	listing = DecodeJSON(t, resp, &softwarecenter.Listing{})
	names := []string{}
	for _, entry := range listing.Entries {
		names = append(names, entry.Name)
	}
	assert.Equal(t, []string{"Android", "iOS", "macOS", "Windows", "全平台"}, names)
}
