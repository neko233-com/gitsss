// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	stdcontext "context"
	"errors"
	"net/http"
	"strconv"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/modules/web/middleware"
	auth_service "github.com/neko233-com/gitsss/services/auth"
	"github.com/neko233-com/gitsss/services/context"
)

func prepareLoginChallenge(ctx *context.Context) {
	ctx.Data["EnableCaptcha"] = true
	ctx.Data["CaptchaType"] = setting.ImageCaptcha
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["LoginProtectionChallenge"] = true
}

func passwordLoginContext(ctx *context.Context, username, password string, tpl templates.TplName, form any, configuredCaptchaPassed bool) (stdcontext.Context, bool) {
	loginCtx := auth_service.WithLoginRequest(ctx, ctx.Req)
	verified := configuredCaptchaPassed
	if !verified && auth_service.LoginChallengeRequired(loginCtx, username, password) {
		prepareLoginChallenge(ctx)
		verified = context.VerifyImageCaptcha(ctx.Req)
		if !verified {
			renderLoginProtectionError(ctx, auth_service.ErrLoginChallenge, tpl, form)
			return nil, false
		}
	}
	return auth_service.WithInteractiveLogin(loginCtx, verified), true
}

func renderLoginProtectionError(ctx *context.Context, err error, tpl templates.TplName, form any) bool {
	status := http.StatusOK
	var message any
	if errors.Is(err, auth_service.ErrLoginChallenge) {
		prepareLoginChallenge(ctx)
		message = ctx.Tr("auth.login_verification_required")
	} else if limit, ok := errors.AsType[*auth_service.LoginRateLimitError](err); ok {
		status = http.StatusTooManyRequests
		ctx.Resp.Header().Set("Retry-After", strconv.Itoa(limit.RetryAfter))
		message = ctx.Tr("auth.login_retry_after", limit.RetryAfter)
	} else {
		return false
	}
	if form != nil {
		middleware.AssignForm(form, ctx.Data)
	}
	delete(ctx.Data, "password")
	ctx.Flash.Error(message, true)
	ctx.HTML(status, tpl)
	return true
}
