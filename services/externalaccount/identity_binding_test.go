// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"testing"

	_ "github.com/neko233-com/gitsss/models"
	"github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func TestVerifiedGoogleEmail(t *testing.T) {
	source := &auth.Source{Cfg: &oauth2.Source{Provider: "gplus"}}
	identity := goth.User{Email: "verified@example.com", RawData: map[string]any{"email_verified": true}}
	assert.Equal(t, identity.Email, verifiedGoogleEmail(source, identity))
	identity.RawData["email_verified"] = "true"
	assert.Empty(t, verifiedGoogleEmail(source, identity))
	identity.RawData["verified_email"] = true
	assert.Equal(t, identity.Email, verifiedGoogleEmail(source, identity))
	source.Cfg = &oauth2.Source{Provider: "feishu"}
	assert.Empty(t, verifiedGoogleEmail(source, identity))
}

func TestIdentityBindingReplacementAndOwnership(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	source := &auth.Source{Type: auth.OAuth2, Name: "Feishu test", IsActive: true, Cfg: &oauth2.Source{Provider: "feishu"}}
	require.NoError(t, auth.CreateSource(t.Context(), source))
	require.NoError(t, BindVerifiedIdentity(t.Context(), source, user, goth.User{UserID: "original"}, false))
	assert.ErrorIs(t, BindVerifiedIdentity(t.Context(), source, other, goth.User{UserID: "original"}, false), ErrIdentityOwned)
	assert.ErrorIs(t, BindVerifiedIdentity(t.Context(), source, user, goth.User{UserID: "new"}, false), ErrProviderBound)
	require.NoError(t, BindVerifiedIdentity(t.Context(), source, user, goth.User{UserID: "new"}, true))
	_, oldExists, err := user_model.GetExternalLogin(t.Context(), source.ID, "original")
	require.NoError(t, err)
	assert.False(t, oldExists)
	link, exists, err := user_model.GetExternalLogin(t.Context(), source.ID, "new")
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, user.ID, link.UserID)
	user.Passwd = ""
	assert.NoError(t, user_model.UpdateUserCols(t.Context(), user, "passwd"))
	originalSetting := setting.Service.EnablePasswordSignInForm
	t.Cleanup(func() { setting.Service.EnablePasswordSignInForm = originalSetting })
	setting.Service.EnablePasswordSignInForm = true
	_, err = UnlinkIdentitySource(t.Context(), user, source.ID)
	assert.Error(t, err, "last login method cannot be removed")
}

func TestGoogleAliasDoesNotOverwritePrimaryOrOtherOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	original := user.Email
	source := &auth.Source{Cfg: &oauth2.Source{Provider: "gplus"}}
	identity := goth.User{Email: "google-alias@example.com", RawData: map[string]any{"verified_email": true}}
	require.NoError(t, SyncGoogleEmail(t.Context(), source, user, identity))
	alias, err := user_model.GetEmailAddressByEmail(t.Context(), identity.Email)
	require.NoError(t, err)
	assert.True(t, alias.IsActivated)
	assert.False(t, alias.IsPrimary)
	assert.Equal(t, original, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}).Email)
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	assert.True(t, user_model.IsErrEmailAlreadyUsed(SyncGoogleEmail(t.Context(), source, other, identity)))
	count, err := db.GetEngine(t.Context()).Where("lower_email=?", identity.Email).Count(new(user_model.EmailAddress))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestIdentityReplacementRetainsOtherProvidersAndRollsBackConflict(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	google := &auth.Source{Type: auth.OAuth2, Name: "Google replacement", IsActive: true, Cfg: &oauth2.Source{Provider: "gplus"}}
	wechat := &auth.Source{Type: auth.OAuth2, Name: "WeChat companion", IsActive: true, Cfg: &oauth2.Source{Provider: "wechat"}}
	require.NoError(t, auth.CreateSource(t.Context(), google))
	require.NoError(t, auth.CreateSource(t.Context(), wechat))
	require.NoError(t, BindVerifiedIdentity(t.Context(), google, user, goth.User{UserID: "google-original"}, false))
	require.NoError(t, BindVerifiedIdentity(t.Context(), wechat, user, goth.User{UserID: "wechat-original"}, false))
	conflict := goth.User{UserID: "google-conflict", Email: other.Email, RawData: map[string]any{"email_verified": true}}
	assert.True(t, user_model.IsErrEmailAlreadyUsed(BindVerifiedIdentity(t.Context(), google, user, conflict, true)))
	_, originalExists, err := user_model.GetExternalLogin(t.Context(), google.ID, "google-original")
	require.NoError(t, err)
	assert.True(t, originalExists)
	_, conflictExists, err := user_model.GetExternalLogin(t.Context(), google.ID, "google-conflict")
	require.NoError(t, err)
	assert.False(t, conflictExists)
	require.NoError(t, BindVerifiedIdentity(t.Context(), google, user, goth.User{UserID: "google-replacement"}, true))
	_, companionExists, err := user_model.GetExternalLogin(t.Context(), wechat.ID, "wechat-original")
	require.NoError(t, err)
	assert.True(t, companionExists)
}
