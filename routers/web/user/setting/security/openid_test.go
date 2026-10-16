// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestDeleteOpenIDReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security?id=1")
	contexttest.LoadUser(t, ctx, 2)
	DeleteOpenID(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}

func TestToggleOpenIDVisibilityReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security?id=1")
	contexttest.LoadUser(t, ctx, 2)
	ToggleOpenIDVisibility(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}
