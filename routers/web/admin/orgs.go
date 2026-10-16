// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/routers/web/explore"
	"github.com/neko233-com/gitsss/services/context"
)

const (
	tplOrgs templates.TplName = "admin/org/list"
)

// Organizations show all the organizations
func Organizations(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.organizations")
	ctx.Data["PageIsAdminOrganizations"] = true

	sortOrder := ctx.FormString("sort", UserSearchDefaultAdminSort)
	explore.RenderUserSearch(ctx, user_model.SearchUserOptions{
		Actor:           ctx.Doer,
		Types:           []user_model.UserType{user_model.UserTypeOrganization},
		IncludeReserved: true, // administrator needs to list all accounts include reserved
		ListOptions: db.ListOptions{
			PageSize: setting.UI.Admin.OrgPagingNum,
		},
		Visible: []structs.VisibleType{structs.VisibleTypePublic, structs.VisibleTypeLimited, structs.VisibleTypePrivate},
		OrderBy: db.SearchOrderBy(sortOrder),
	}, tplOrgs)
}
