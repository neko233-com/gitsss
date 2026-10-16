// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net"
	"net/http"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/listenport"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
)

// PostListenPortAlias adds an HTTP listener while keeping existing ports open.
func PostListenPortAlias(ctx *context.APIContext) {
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
	port, err := listenport.ValidatePort(body.Port)
	if err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	if err := listenport.AddAlias(port); err != nil {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	}
	ports := append([]string{setting.HTTPPort}, setting.HTTPAdditionalPorts...)
	ctx.JSON(http.StatusOK, map[string]any{
		"ports":    ports,
		"root_url": setting.AppURL,
		"address":  net.JoinHostPort(setting.HTTPAddr, port),
	})
}
