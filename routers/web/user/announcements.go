// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	"github.com/neko233-com/gitsss/modules/announcement"
	"github.com/neko233-com/gitsss/services/context"
)

// Announcements returns published enterprise notices for the signed-in user.
func Announcements(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	notices, err := announcement.Load(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "announcements unavailable"})
		return
	}
	ctx.JSON(http.StatusOK, announcement.Published(notices))
}
