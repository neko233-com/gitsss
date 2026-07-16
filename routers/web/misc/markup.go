// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	api "github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/modules/util"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/routers/common"
	"github.com/neko233-com/gitsss/services/context"
)

// Markup render markup document to HTML
func Markup(ctx *context.Context) {
	form := web.GetForm(ctx).(*api.MarkupOption)
	mode := util.Iif(form.Wiki, "wiki", form.Mode) //nolint:staticcheck // form.Wiki is deprecated
	common.RenderMarkup(ctx.Base, ctx.Repo, mode, form.Text, form.Context, form.FilePath)
}
