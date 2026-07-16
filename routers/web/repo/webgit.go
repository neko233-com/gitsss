// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/util"
	"github.com/neko233-com/gitsss/services/context"
	files_service "github.com/neko233-com/gitsss/services/repository/files"
)

func WebGitOperationCommonData(ctx *context.Context) {
	// TODO: more places like "wiki page" and "merging a pull request or creating an auto merge merging task"
	emails, err := user_model.GetActivatedEmailAddresses(ctx, ctx.Doer.ID)
	if err != nil {
		log.Error("WebGitOperationCommonData: GetActivatedEmailAddresses: %v", err)
	}
	if ctx.Doer.KeepEmailPrivate {
		emails = append([]string{ctx.Doer.GetPlaceholderEmail()}, emails...)
	}
	ctx.Data["CommitCandidateEmails"] = emails
	ctx.Data["CommitDefaultEmail"] = ctx.Doer.GetEmail()
}

func WebGitOperationGetCommitChosenEmailIdentity(ctx *context.Context, email string) (_ *files_service.IdentityOptions, valid bool) {
	if ctx.Data["CommitCandidateEmails"] == nil {
		setting.PanicInDevOrTesting("no CommitCandidateEmails in context data")
	}
	emails, _ := ctx.Data["CommitCandidateEmails"].([]string)
	if email == "" {
		return nil, true
	}
	if util.SliceContainsString(emails, email, true) {
		return &files_service.IdentityOptions{GitUserEmail: email}, true
	}
	return nil, false
}
