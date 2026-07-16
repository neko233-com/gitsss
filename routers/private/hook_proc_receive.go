// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"errors"
	"net/http"

	issues_model "github.com/neko233-com/gitsss/models/issues"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/private"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/agit"
	gitea_context "github.com/neko233-com/gitsss/services/context"
)

// HookProcReceive proc-receive hook - only handles agit Proc-Receive requests at present
func HookProcReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)
	if !git.DefaultFeatures().SupportProcReceive {
		ctx.Status(http.StatusNotFound)
		return
	}

	results, err := agit.ProcReceive(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, opts)
	if err != nil {
		if errors.Is(err, issues_model.ErrMustCollaborator) {
			ctx.JSON(http.StatusUnauthorized, private.Response{
				Err: err.Error(), UserMsg: "You must be a collaborator to create pull request.",
			})
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSON(http.StatusUnauthorized, private.Response{
				Err: err.Error(), UserMsg: "Cannot create pull request because you are blocked by the repository owner.",
			})
		} else {
			log.Error("agit.ProcReceive failed: %v", err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
		}

		return
	}

	ctx.JSON(http.StatusOK, private.HookProcReceiveResult{
		Results: results,
	})
}
