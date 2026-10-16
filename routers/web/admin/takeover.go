// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"time"

	"github.com/neko233-com/gitsss/modules/announcement"
	"github.com/neko233-com/gitsss/modules/listenport"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/forms"
)

const tplTakeover templates.TplName = "admin/takeover"

// Takeover shows the port switch used to replace an existing Gitea listener.
func Takeover(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.takeover")
	ctx.Data["PageIsAdminTakeover"] = true
	ctx.Data["HTTPPort"] = setting.HTTPPort
	ctx.Data["HTTPAdditionalPorts"] = setting.HTTPAdditionalPorts
	ctx.Data["AppURL"] = setting.AppURL
	if notices, err := announcement.Load(ctx); err == nil {
		ctx.Data["Announcements"] = notices
	}
	ctx.HTML(http.StatusOK, tplTakeover)
}

// TakeoverPost saves and rebinds the HTTP port.
func TakeoverPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminTakeoverForm)
	var err error
	if form.Mode == "add-alias" {
		err = listenport.AddAlias(form.Port)
	} else {
		err = listenport.Apply(form.Port)
	}
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
	} else if form.Mode == "add-alias" {
		ctx.Flash.Success(ctx.Tr("admin.takeover.alias_saved", form.Port, setting.AppURL))
	} else {
		ctx.Flash.Success(ctx.Tr("admin.takeover.saved", setting.HTTPPort, setting.AppURL))
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/takeover")
}

// AnnouncementsPost publishes one enterprise announcement for signed-in users.
func AnnouncementsPost(ctx *context.Context) {
	notices, err := announcement.Load(ctx)
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
		ctx.Redirect(setting.AppSubURL + "/-/admin/takeover")
		return
	}
	notices, err = announcement.Upsert(notices, announcement.Notice{
		Title: ctx.FormString("title"), Body: ctx.FormString("body"), Published: true,
	}, time.Now())
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
		ctx.Redirect(setting.AppSubURL + "/-/admin/takeover")
		return
	}
	if err := announcement.Save(ctx, notices); err != nil {
		ctx.Flash.Error(err.Error(), true)
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/takeover")
}
