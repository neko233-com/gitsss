// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/optional"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/web"
	auth_service "github.com/neko233-com/gitsss/services/auth"
	web_context "github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/forms"
)

const (
	tplAccountInvite            templates.TplName = "user/auth/account_invite"
	accountInvitePasswordMinLen int               = 6
)

func prepareAccountInvitePage(ctx *web_context.Context, token string) bool {
	if !auth_service.IsAccountInviteToken(token) {
		ctx.NotFound(nil)
		return false
	}
	if _, err := auth_service.CheckAuthToken(ctx, token); err != nil {
		ctx.NotFound(nil)
		return false
	}

	ctx.Data["Title"] = ctx.Tr("auth.account_invite_title")
	ctx.Data["AccountInviteToken"] = token
	ctx.Data["AccountInvitePasswordMinLength"] = accountInvitePasswordMinLen
	return true
}

// AccountInvite renders the one-time account setup form.
func AccountInvite(ctx *web_context.Context) {
	if !prepareAccountInvitePage(ctx, ctx.PathParam("token")) {
		return
	}
	ctx.HTML(http.StatusOK, tplAccountInvite)
}

// AccountInvitePost creates an active local account and consumes the invitation atomically.
func AccountInvitePost(ctx *web_context.Context) {
	token := ctx.PathParam("token")
	if !prepareAccountInvitePage(ctx, token) {
		return
	}

	form := web.GetForm(ctx).(*forms.RegisterForm)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplAccountInvite)
		return
	}
	if !form.IsEmailDomainAllowed() {
		ctx.RenderWithErrDeprecated(ctx.Tr("auth.email_domain_blacklisted"), tplAccountInvite, &form)
		return
	}
	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErrDeprecated(ctx.Tr("form.password_not_match"), tplAccountInvite, &form)
		return
	}
	if len(form.Password) < accountInvitePasswordMinLen {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErrDeprecated(ctx.Tr("auth.account_invite_password_too_short", accountInvitePasswordMinLen), tplAccountInvite, &form)
		return
	}

	u := &user_model.User{
		Name:   form.UserName,
		Email:  form.Email,
		Passwd: form.Password,
	}
	err := db.WithTx(ctx, func(txCtx context.Context) error {
		if err := auth_service.ConsumeAccountInviteToken(txCtx, token); err != nil {
			return err
		}
		return user_model.CreateUser(txCtx, u, &user_model.Meta{
			InitialIP:        ctx.RemoteAddr(),
			InitialUserAgent: ctx.Req.UserAgent(),
		}, &user_model.CreateUserOverwriteOptions{
			IsActive: optional.Some(true),
		})
	})
	if err != nil {
		switch {
		case errors.Is(err, auth_service.ErrAccountInviteInvalid):
			ctx.RenderWithErrDeprecated(ctx.Tr("auth.account_invite_invalid"), tplAccountInvite, &form)
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("form.username_been_taken"), tplAccountInvite, &form)
		case user_model.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("form.email_been_used"), tplAccountInvite, &form)
		case user_model.IsErrEmailCharIsNotSupported(err), user_model.IsErrEmailInvalid(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("form.email_invalid"), tplAccountInvite, &form)
		case db.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("user.form.name_reserved", err.(db.ErrNameReserved).Name), tplAccountInvite, &form)
		case db.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("user.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplAccountInvite, &form)
		case db.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErrDeprecated(ctx.Tr("user.form.name_chars_not_allowed", strings.TrimSpace(form.UserName)), tplAccountInvite, &form)
		default:
			ctx.ServerError("CreateInvitedUser", err)
		}
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.account_invite_successful"))
	handleSignIn(ctx, u, false)
}
