// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"

	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/optional"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/convert"
)

// SearchCandidates searches candidate users for dropdown list
func SearchCandidates(ctx *context.Context) {
	searchUserTypes := []user_model.UserType{user_model.UserTypeIndividual}
	if ctx.FormBool("orgs") {
		searchUserTypes = append(searchUserTypes, user_model.UserTypeOrganization)
	}
	users, _, err := user_model.SearchUsers(ctx, user_model.SearchUserOptions{
		Actor:    ctx.Doer,
		Keyword:  ctx.FormTrim("q"),
		Types:    searchUserTypes,
		IsActive: optional.Some(true),
		PageSize: setting.UI.MembersPagingNum,
	})
	if err != nil {
		ctx.ServerError("Unable to search users", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"data": convert.ToUsers(ctx, ctx.Doer, users)})
}
