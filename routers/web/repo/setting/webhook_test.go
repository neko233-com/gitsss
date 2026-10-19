// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"testing"

	"github.com/neko233-com/gitsss/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestNotificationValidationErrorCurrentPage(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "/org/example/settings/hooks/email/new")
	showNotificationSettingsError(ctx, errors.New("unknown template variable"))
	assert.Same(t, ctx.Flash, ctx.Data["Flash"])
	assert.Contains(t, ctx.Flash.ErrorMsg, "unknown template variable")
	assert.Empty(t, ctx.Flash.Get("error"), "do not defer the error to an unrelated next page")
}

func TestWebhookMatrix(t *testing.T) {
	assert.Equal(t, "!roomid:domain", matrixRoomIDEncode("!roomid:domain"))
	assert.Equal(t, "!room%23id:domain", matrixRoomIDEncode("!room#id:domain")) // maybe it should never really happen in real world
}
