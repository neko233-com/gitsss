// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"errors"
	"strings"

	"github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/util"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"

	"github.com/markbates/goth"
)

var (
	ErrLastIdentity  = errors.New("keep at least one working sign-in method before unlinking")
	ErrIdentityOwned = errors.New("this identity is already bound to another account")
	ErrProviderBound = errors.New("this provider is already bound; use replace to verify a new identity")
)

func NativeIdentitySource(source *auth.Source) bool {
	config, ok := source.Cfg.(*oauth2.Source)
	return ok && oauth2.IsNativeIdentityProvider(config.Provider)
}

func identitySourceUsable(source *auth.Source) bool {
	cfg, oauth := source.Cfg.(*oauth2.Source)
	return source.IsActive && (!oauth || cfg.IsConfigured())
}

// UnlinkIdentitySource retains a usable login method and removes a legacy primary OAuth identity too.
func UnlinkIdentitySource(ctx context.Context, user *user_model.User, sourceID int64) (int64, error) {
	var deleted int64
	err := db.WithTx(ctx, func(ctx context.Context) error {
		source, err := auth.GetSourceByID(ctx, sourceID)
		if err != nil {
			return err
		}
		if !NativeIdentitySource(source) {
			deleted, err = user_model.RemoveAccountLink(ctx, user, sourceID)
			return err
		}
		if _, err := db.GetEngine(ctx).NoAutoTime().ID(user.ID).SetExpr("id", "id").Update(new(user_model.User)); err != nil {
			return err
		}
		user, err = user_model.GetUserByID(ctx, user.ID)
		if err != nil {
			return err
		}
		usable := setting.Service.EnablePasswordSignInForm && user.IsPasswordSet()
		links, err := db.Find[user_model.ExternalLoginUser](ctx, user_model.FindExternalUserOptions{UserID: user.ID})
		if err != nil {
			return err
		}
		for _, link := range links {
			if link.LoginSourceID == sourceID {
				continue
			}
			other, err := auth.GetSourceByID(ctx, link.LoginSourceID)
			if err == nil && identitySourceUsable(other) {
				usable = true
			}
		}
		if user.LoginSource > 0 && user.LoginSource != sourceID {
			other, err := auth.GetSourceByID(ctx, user.LoginSource)
			if err == nil && identitySourceUsable(other) {
				usable = true
			}
		}
		if !usable {
			return ErrLastIdentity
		}
		if user.LoginType == auth.OAuth2 && user.LoginSource == sourceID {
			updated := *user
			updated.LoginType, updated.LoginSource, updated.LoginName = auth.Plain, 0, ""
			if err := user_model.UpdateUserCols(ctx, &updated, "login_type", "login_source", "login_name"); err != nil {
				return err
			}
		}
		deleted, err = user_model.RemoveAccountLink(ctx, user, sourceID)
		return err
	})
	return deleted, err
}

func verifiedGoogleEmail(source *auth.Source, identity goth.User) string {
	config, ok := source.Cfg.(*oauth2.Source)
	if !ok || config.Provider != "gplus" {
		return ""
	}
	verified, _ := identity.RawData["email_verified"].(bool)
	legacyVerified, _ := identity.RawData["verified_email"].(bool)
	if !verified && !legacyVerified {
		return ""
	}
	return strings.TrimSpace(identity.Email)
}

// SyncGoogleEmail adds an independently verified alias; the user's primary address stays their choice.
func SyncGoogleEmail(ctx context.Context, source *auth.Source, user *user_model.User, identity goth.User) error {
	email := verifiedGoogleEmail(source, identity)
	if email == "" {
		return nil
	}
	if err := user_model.ValidateEmail(email); err != nil {
		return err
	}
	alias, err := user_model.GetEmailAddressByEmail(ctx, email)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}
	if alias != nil {
		if alias.UID != user.ID {
			return user_model.ErrEmailAlreadyUsed{Email: email}
		}
		if !alias.IsActivated {
			return user_model.ActivateEmail(ctx, alias)
		}
		return nil
	}
	_, err = user_model.InsertEmailAddress(ctx, &user_model.EmailAddress{UID: user.ID, Email: email, IsActivated: true})
	return err
}

// BindVerifiedIdentity replaces only this provider after its new identity was authenticated.
func BindVerifiedIdentity(ctx context.Context, source *auth.Source, user *user_model.User, identity goth.User, replace bool) error {
	if identity.UserID == "" || !NativeIdentitySource(source) {
		return util.NewInvalidArgumentErrorf("invalid identity")
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		// Serialize changes for this account without overwriting its profile or timestamps.
		if _, err := db.GetEngine(ctx).NoAutoTime().ID(user.ID).SetExpr("id", "id").Update(new(user_model.User)); err != nil {
			return err
		}
		existing, found, err := user_model.GetExternalLogin(ctx, source.ID, identity.UserID)
		if err != nil {
			return err
		}
		if found && existing.UserID != user.ID {
			return ErrIdentityOwned
		}
		links, err := db.Find[user_model.ExternalLoginUser](ctx, user_model.FindExternalUserOptions{UserID: user.ID, LoginSourceID: source.ID})
		if err != nil {
			return err
		}
		for _, link := range links {
			if link.ExternalID != identity.UserID && !replace {
				return ErrProviderBound
			}
		}
		if err := SyncGoogleEmail(ctx, source, user, identity); err != nil {
			return err
		}
		if err := user_model.EnsureLinkExternalToUser(ctx, toExternalLoginUser(source.ID, user, identity)); err != nil {
			return err
		}
		if replace {
			_, err = db.GetEngine(ctx).Where("user_id=? AND login_source_id=? AND external_id<>?", user.ID, source.ID, identity.UserID).Delete(new(user_model.ExternalLoginUser))
			if err != nil {
				return err
			}
			if user.LoginType == auth.OAuth2 && user.LoginSource == source.ID {
				updated := *user
				updated.LoginName = identity.UserID
				return user_model.UpdateUserCols(ctx, &updated, "login_name")
			}
		}
		return nil
	})
}
