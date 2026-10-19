// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"testing"

	"github.com/neko233-com/gitsss/models/asymkey"
	"github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/models/perm/access"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func identityLifecycleRows[T any](t *testing.T) []*T {
	t.Helper()
	var rows []*T
	require.NoError(t, db.GetEngine(t.Context()).Asc("id").Find(&rows))
	return rows
}

func identityLifecycleAccountState(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"accounts":      identityLifecycleRows[user_model.User](t),
		"ssh_keys":      identityLifecycleRows[asymkey.PublicKey](t),
		"deploy_keys":   identityLifecycleRows[asymkey.DeployKey](t),
		"tokens":        identityLifecycleRows[auth.AccessToken](t),
		"repo_access":   identityLifecycleRows[access.Access](t),
		"collaborators": identityLifecycleRows[repo_model.Collaboration](t),
		"org_members":   identityLifecycleRows[organization.OrgUser](t),
		"team_members":  identityLifecycleRows[organization.TeamUser](t),
		"team_repos":    identityLifecycleRows[organization.TeamRepo](t),
	}
}

func TestFeishuLifecyclePreservesAccountsCredentialsAndPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	originalPasswordSetting := setting.Service.EnablePasswordSignInForm
	t.Cleanup(func() { setting.Service.EnablePasswordSignInForm = originalPasswordSetting })
	setting.Service.EnablePasswordSignInForm = true
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	feishu := &auth.Source{Type: auth.OAuth2, Name: "Feishu lifecycle", IsActive: true, Cfg: &oauth2.Source{Provider: "feishu", ClientID: "fixture", ClientSecret: "fixture"}}
	companion := &auth.Source{Type: auth.OAuth2, Name: "Companion lifecycle", IsActive: true, Cfg: &oauth2.Source{Provider: "wechat", ClientID: "fixture", ClientSecret: "fixture"}}
	require.NoError(t, auth.CreateSource(t.Context(), feishu))
	require.NoError(t, auth.CreateSource(t.Context(), companion))
	require.NoError(t, BindVerifiedIdentity(t.Context(), companion, user, goth.User{UserID: "companion"}, false))
	require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, other, goth.User{UserID: "occupied"}, false))
	before := identityLifecycleAccountState(t)
	assertOwner := func(id string, owner int64, exists bool) {
		t.Helper()
		link, found, err := user_model.GetExternalLogin(t.Context(), feishu.ID, id)
		require.NoError(t, err)
		require.Equal(t, exists, found)
		if found {
			assert.Equal(t, owner, link.UserID)
		}
		assert.Equal(t, before, identityLifecycleAccountState(t))
	}

	require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "first"}, false))
	require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "first"}, false))
	assertOwner("first", user.ID, true)
	assert.ErrorIs(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "second"}, false), ErrProviderBound)
	assertOwner("first", user.ID, true)
	require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "second"}, true))
	assertOwner("first", 0, false)
	assertOwner("second", user.ID, true)
	assert.ErrorIs(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "occupied"}, true), ErrIdentityOwned)
	assertOwner("second", user.ID, true)
	assertOwner("occupied", other.ID, true)
	deleted, err := UnlinkIdentitySource(t.Context(), user, feishu.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	assertOwner("second", 0, false)
	_, err = UnlinkIdentitySource(t.Context(), user, feishu.ID)
	var notLinked user_model.ErrExternalLoginUserNotExist
	assert.ErrorAs(t, err, &notLinked)
	require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "first"}, false))
	assertOwner("first", user.ID, true)
	link, found, err := user_model.GetExternalLogin(t.Context(), companion.ID, "companion")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, user.ID, link.UserID)
}

func TestFeishuUnlinkRequiresWorkingFallback(t *testing.T) {
	for _, name := range []string{"local password", "missing password", "disabled password form", "configured companion", "inactive companion", "incomplete companion"} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			originalPasswordSetting := setting.Service.EnablePasswordSignInForm
			t.Cleanup(func() { setting.Service.EnablePasswordSignInForm = originalPasswordSetting })
			setting.Service.EnablePasswordSignInForm = name != "disabled password form"
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			if name != "local password" && name != "disabled password form" {
				user.Passwd = ""
				require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "passwd"))
			}
			feishu := &auth.Source{Type: auth.OAuth2, Name: "Feishu fallback", IsActive: true, Cfg: &oauth2.Source{Provider: "feishu", ClientID: "fixture", ClientSecret: "fixture"}}
			require.NoError(t, auth.CreateSource(t.Context(), feishu))
			require.NoError(t, BindVerifiedIdentity(t.Context(), feishu, user, goth.User{UserID: "original"}, false))
			if name == "configured companion" || name == "inactive companion" || name == "incomplete companion" {
				cfg := &oauth2.Source{Provider: "discord", ClientID: "fixture", ClientSecret: "fixture"}
				if name == "incomplete companion" {
					cfg.ClientSecret = ""
				}
				companion := &auth.Source{Type: auth.OAuth2, Name: "Fallback companion", IsActive: name != "inactive companion", Cfg: cfg}
				require.NoError(t, auth.CreateSource(t.Context(), companion))
				require.NoError(t, BindVerifiedIdentity(t.Context(), companion, user, goth.User{UserID: "companion"}, false))
			}
			before := identityLifecycleAccountState(t)
			_, err := UnlinkIdentitySource(t.Context(), user, feishu.ID)
			allowed := name == "local password" || name == "configured companion"
			if allowed {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrLastIdentity)
			}
			_, found, err := user_model.GetExternalLogin(t.Context(), feishu.ID, "original")
			require.NoError(t, err)
			assert.Equal(t, !allowed, found)
			assert.Equal(t, before, identityLifecycleAccountState(t))
		})
	}
}
