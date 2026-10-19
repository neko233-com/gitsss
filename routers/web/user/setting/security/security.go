// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"errors"
	"net/http"
	"sort"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/optional"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/externalaccount"
)

const (
	tplSettingsSecurity    templates.TplName = "user/settings/security/security"
	tplSettingsTwofaEnroll templates.TplName = "user/settings/security/twofa_enroll"
)

// Security render change user's password page and 2FA
func Security(ctx *context.Context) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer,
		setting.UserFeatureManageMFA, setting.UserFeatureManageCredentials) {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	ctx.Data["Title"] = ctx.Tr("settings.security")
	ctx.Data["PageIsSettingsSecurity"] = true

	if ctx.FormString("openid.return_to") != "" {
		settingsOpenIDVerify(ctx)
		return
	}

	loadSecurityData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsSecurity)
}

// DeleteAccountLink delete a single account link
func DeleteAccountLink(ctx *context.Context) {
	if user_model.IsFeatureDisabledWithLoginType(ctx.Doer, setting.UserFeatureManageCredentials) {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	id := ctx.FormInt64("id")
	if id <= 0 {
		ctx.Flash.Error("Account link id is not given")
	} else {
		if _, err := externalaccount.UnlinkIdentitySource(ctx, ctx.Doer, id); err != nil {
			if errors.Is(err, externalaccount.ErrLastIdentity) {
				ctx.Flash.Error(ctx.Tr("settings.identity_last_method"))
			} else {
				ctx.Flash.Error(ctx.Tr("settings.identity_binding_failed"))
			}
		} else {
			ctx.Flash.Success(ctx.Tr("settings.remove_account_link_success"))
		}
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/security")
}

func loadSecurityData(ctx *context.Context) {
	enrolled, err := auth_model.HasTwoFactorByUID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("SettingsTwoFactor", err)
		return
	}
	ctx.Data["TOTPEnrolled"] = enrolled

	credentials, err := auth_model.GetWebAuthnCredentialsByUID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetWebAuthnCredentialsByUID", err)
		return
	}
	ctx.Data["WebAuthnCredentials"] = credentials

	tokens, err := db.Find[auth_model.AccessToken](ctx, auth_model.ListAccessTokensOptions{UserID: ctx.Doer.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens

	accountLinks, err := db.Find[user_model.ExternalLoginUser](ctx, user_model.FindExternalUserOptions{
		UserID:  ctx.Doer.ID,
		OrderBy: "login_source_id DESC",
	})
	if err != nil {
		ctx.ServerError("ListAccountLinks", err)
		return
	}

	// map the provider display name with the AuthSource
	sources := make(map[*auth_model.Source]string)
	identities := map[int64]*user_model.ExternalLoginUser{}
	for _, externalAccount := range accountLinks {
		identities[externalAccount.LoginSourceID] = externalAccount
		if authSource, err := auth_model.GetSourceByID(ctx, externalAccount.LoginSourceID); err == nil {
			var providerDisplayName string

			type DisplayNamed interface {
				DisplayName() string
			}

			type Named interface {
				Name() string
			}

			if displayNamed, ok := authSource.Cfg.(DisplayNamed); ok {
				providerDisplayName = displayNamed.DisplayName()
			} else if named, ok := authSource.Cfg.(Named); ok {
				providerDisplayName = named.Name()
			} else {
				providerDisplayName = authSource.Name
			}
			sources[authSource] = providerDisplayName
		}
	}
	ctx.Data["AccountLinks"] = sources
	ctx.Data["HasLocalPassword"] = setting.Service.EnablePasswordSignInForm && ctx.Doer.IsPasswordSet()
	ctx.Data["ExternalIdentityDetails"] = identities

	authSources, err := db.Find[auth_model.Source](ctx, auth_model.FindSourcesOptions{
		IsActive:  optional.None[bool](),
		LoginType: auth_model.OAuth2,
	})
	if err != nil {
		ctx.ServerError("FindSources", err)
		return
	}

	nativeSources := map[int64]bool{}
	var orderedOAuth2Names []string
	oauth2Providers := make(map[string]oauth2.Provider)
	for _, source := range authSources {
		nativeSources[source.ID] = externalaccount.NativeIdentitySource(source) && source.IsActive && source.Cfg.(*oauth2.Source).IsConfigured()
		provider, err := oauth2.CreateProviderFromSource(source)
		if err != nil {
			ctx.ServerError("CreateProviderFromSource", err)
			return
		}
		oauth2Providers[source.Name] = provider
		if source.IsActive && source.Cfg.(*oauth2.Source).IsConfigured() && (!nativeSources[source.ID] || identities[source.ID] == nil) {
			orderedOAuth2Names = append(orderedOAuth2Names, source.Name)
		}
	}

	sort.Strings(orderedOAuth2Names)

	ctx.Data["NativeIdentitySources"] = nativeSources
	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers

	openid, err := user_model.GetUserOpenIDs(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openid
}
