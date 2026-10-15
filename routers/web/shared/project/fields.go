// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"net/http"

	project_model "github.com/neko233-com/gitsss/models/project"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/services/context"
)

// SaveFields stores custom board fields. The request body is a JSON array in the form field "fields".
func SaveFields(ctx *context.Context) {
	board, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	var repository *repo_model.Repository
	if ctx.Repo != nil {
		repository = ctx.Repo.Repository
	}
	ownerID := int64(0)
	if ctx.ContextUser != nil {
		ownerID = ctx.ContextUser.ID
	}
	if !board.CanBeAccessedByOwnerRepo(ownerID, repository) {
		ctx.NotFound(nil)
		return
	}
	var fields []project_model.Field
	raw := ctx.FormString("fields")
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			ctx.Flash.Error(ctx.Tr("projects.fields_invalid"), true)
			ctx.Redirect(board.Link(ctx))
			return
		}
	}
	encoded, err := project_model.EncodeFields(fields)
	if err != nil {
		ctx.Flash.Error(err.Error(), true)
		ctx.Redirect(board.Link(ctx))
		return
	}
	board.FieldsJSON = encoded
	if err := project_model.UpdateProject(ctx, board); err != nil {
		ctx.ServerError("UpdateProject", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("projects.fields_saved"))
	ctx.Redirect(board.Link(ctx))
	_ = http.StatusOK
}
