// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/homelinks"
	"github.com/neko233-com/gitsss/modules/system"
	"github.com/neko233-com/gitsss/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomeLinksAuthority(t *testing.T) {
	require.NoError(t, unittest.LoadFixtures())
	previous := system.AppState
	system.AppState = &system.DBStore{}
	t.Cleanup(func() { system.AppState = previous })
	ctx, _ := contexttest.MockContext(t, "POST /home-links")
	contexttest.LoadUser(t, ctx, 2)
	ctx.IsSigned = true
	ctx.Doer.IsAdmin = false
	ctx.Req.Form.Set("scope", "shared")
	HomeLinksPost(ctx)
	assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())

	ctx, _ = contexttest.MockContext(t, "POST /home-links")
	contexttest.LoadUser(t, ctx, 2)
	ctx.IsSigned = true
	state, err := homelinks.Load(ctx, ctx.Doer.ID)
	require.NoError(t, err)
	ctx.Req.Form.Set("scope", "personal")
	ctx.Req.Form.Set("revision", state.Revision())
	ctx.Req.Form.Set("title", "My CI")
	ctx.Req.Form.Set("url", "http://ci.internal:8080")
	ctx.Req.Form.Set("owner_id", "1")
	HomeLinksPost(ctx)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	saved, err := homelinks.Load(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, []homelinks.Link{{Title: "My CI", URL: "http://ci.internal:8080"}}, saved.Links)
	other, err := homelinks.Load(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, other.Links)
	shared, err := homelinks.Load(ctx, 0)
	require.NoError(t, err)
	assert.Empty(t, shared.Links)
}

func TestHomeLinksPagePost(t *testing.T) {
	require.NoError(t, unittest.LoadFixtures())
	previous := system.AppState
	system.AppState = &system.DBStore{}
	t.Cleanup(func() { system.AppState = previous })
	ctx, _ := contexttest.MockContext(t, "POST /home-links")
	contexttest.LoadUser(t, ctx, 2)
	ctx.IsSigned = true
	state, err := homelinks.Load(ctx, 2)
	require.NoError(t, err)
	links := make([]homelinks.Link, 30)
	for i := range links {
		links[i] = homelinks.Link{Title: fmt.Sprintf("Link %d", i), URL: "https://example.test"}
	}
	require.NoError(t, homelinks.Save(ctx, 2, state.Revision(), links))
	state, err = homelinks.Load(ctx, 2)
	require.NoError(t, err)
	ctx.Req.Form.Set("scope", "personal")
	ctx.Req.Form.Set("operation", "page")
	ctx.Req.Form.Set("revision", state.Revision())
	ctx.Req.Form.Set("return_path", "/org/org3/dashboard")
	ctx.Req.Form.Set("links_page", "2")
	ctx.Req.Form.Add("page_index", "24")
	ctx.Req.Form.Add("page_index", "25")
	ctx.Req.Form.Add("item_index", "24")
	ctx.Req.Form.Add("title", "Changed")
	ctx.Req.Form.Add("url", "https://changed.test")
	ctx.Req.Form.Add("item_index", "25")
	ctx.Req.Form.Add("title", "To remove")
	ctx.Req.Form.Add("url", "https://removed.test")
	ctx.Req.Form.Add("delete_index", "25")
	ctx.Req.Form.Add("item_index", "-1")
	ctx.Req.Form.Add("title", "")
	ctx.Req.Form.Add("url", "")
	HomeLinksPost(ctx)
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Equal(t, "/org/org3/dashboard?links_page=2&tab=links", ctx.Resp.Header().Get("Location"))
	saved, err := homelinks.Load(ctx, 2)
	require.NoError(t, err)
	require.Len(t, saved.Links, 29)
	assert.Equal(t, links[23], saved.Links[23])
	assert.Equal(t, "Changed", saved.Links[24].Title)
	assert.Equal(t, links[26], saved.Links[25])
	previousRevision := saved.LoadedRevision()
	saved.Links[0].Title = "Changed by another writer"
	require.NoError(t, system.AppState.Set(ctx, saved))
	updated, err := homelinks.Load(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, "Changed by another writer", updated.Links[0].Title)
	assert.NotEqual(t, previousRevision, updated.LoadedRevision())
	assert.ErrorIs(t, homelinks.Save(ctx, 2, previousRevision, links), homelinks.ErrConflict)

	ctx, _ = contexttest.MockContext(t, "POST /home-links")
	ctx.Req.Form.Set("return_path", "//evil.example/")
	assert.Equal(t, "/", homeLinksReturnPath(ctx))
}
