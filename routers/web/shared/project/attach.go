// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	issues_model "github.com/neko233-com/gitsss/models/issues"
	access_model "github.com/neko233-com/gitsss/models/perm/access"
	project_model "github.com/neko233-com/gitsss/models/project"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/projectboard"
	"github.com/neko233-com/gitsss/services/context"
)

// AttachIssue puts owner/repo#123 onto a user or organization board.
// The issue also stays on any repository board it already belongs to.
func AttachIssue(ctx *context.Context) {
	board, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if board.RepoID != 0 || ctx.ContextUser == nil || board.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound(nil)
		return
	}
	ref, err := projectboard.ParseIssueRef(ctx.FormString("issue"))
	if err != nil {
		ctx.Flash.Error(ctx.Tr("projects.attach_invalid"), true)
		ctx.Redirect(board.Link(ctx))
		return
	}
	repository, err := repo_model.GetRepositoryByOwnerAndName(ctx, ref.Owner, ref.Repo)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error(ctx.Tr("projects.attach_missing"), true)
			ctx.Redirect(board.Link(ctx))
			return
		}
		ctx.ServerError("GetRepositoryByOwnerAndName", err)
		return
	}
	if repository.OwnerID != board.OwnerID || !board.CanBeAccessedByOwnerRepo(repository.OwnerID, repository) {
		ctx.Flash.Error(ctx.Tr("projects.attach_owner"), true)
		ctx.Redirect(board.Link(ctx))
		return
	}
	issue, err := issues_model.GetIssueByIndex(ctx, repository.ID, ref.Number)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.Flash.Error(ctx.Tr("projects.attach_missing"), true)
			ctx.Redirect(board.Link(ctx))
			return
		}
		ctx.ServerError("GetIssueByIndex", err)
		return
	}
	permission, err := access_model.GetDoerRepoPermission(ctx, repository, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetDoerRepoPermission", err)
		return
	}
	if !permission.CanReadIssuesOrPulls(issue.IsPull) {
		ctx.Flash.Error(ctx.Tr("projects.attach_forbidden"), true)
		ctx.Redirect(board.Link(ctx))
		return
	}
	issue.Repo = repository
	if err := issue.LoadProjects(ctx); err != nil {
		ctx.ServerError("LoadProjects", err)
		return
	}
	projectIDs := make([]int64, 0, len(issue.Projects)+1)
	already := false
	for _, existing := range issue.Projects {
		projectIDs = append(projectIDs, existing.ID)
		if existing.ID == board.ID {
			already = true
		}
	}
	if !already {
		projectIDs = append(projectIDs, board.ID)
	}
	if err := issues_model.IssueAssignOrRemoveProject(ctx, issue, ctx.Doer, projectIDs); err != nil {
		ctx.ServerError("IssueAssignOrRemoveProject", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("projects.attach_saved", ref.Owner, ref.Repo, ref.Number))
	ctx.Redirect(board.Link(ctx))
}
