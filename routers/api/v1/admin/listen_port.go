// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/listenport"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
)

// PostListenPort rebinds the HTTP port for an authenticated site administrator.
func PostListenPort(ctx *context.APIContext) {
	if !ctx.IsUserSiteAdmin() {
		ctx.APIError(http.StatusForbidden, "user should be the site admin")
		return
	}
	var body struct {
		Port string `json:"port"`
	}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&body); err != nil {
		ctx.APIError(http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := listenport.Apply(body.Port); err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{
		"port":     setting.HTTPPort,
		"root_url": setting.AppURL,
	})
}
