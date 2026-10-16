// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
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
