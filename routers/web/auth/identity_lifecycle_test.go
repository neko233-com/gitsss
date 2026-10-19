// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"
	"time"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/session"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"
	"github.com/neko233-com/gitsss/services/contexttest"
	"github.com/neko233-com/gitsss/services/externalaccount"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeishuReplacementCallbackRetainsOriginalAccount(t *testing.T) {
	for _, tc := range []struct {
		name            string
		age             time.Duration
		missingIntent   bool
		wrongUser       bool
		wrongSource     bool
		signedOut       bool
		foreignIdentity bool
		valid           bool
	}{
		{name: "verified replacement and replay", valid: true},
		{name: "missing binding intent", missingIntent: true},
		{name: "different signed-in account", wrongUser: true},
		{name: "different source", wrongSource: true},
		{name: "expired intent", age: -11 * time.Minute},
		{name: "future intent", age: time.Minute},
		{name: "session signed out", signedOut: true},
		{name: "identity belongs to another account", foreignIdentity: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			require.NoError(t, oauth2.Init(t.Context()))
			const sourceName = "feishu-lifecycle-fixture"
			addOAuth2Source(t, sourceName, oauth2.Source{Provider: "feishu", ClientID: "fixture", ClientSecret: "fixture"})
			source, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), sourceName)
			require.NoError(t, err)
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
			require.NoError(t, externalaccount.BindVerifiedIdentity(t.Context(), source, user, goth.User{UserID: "old"}, false))
			if tc.foreignIdentity {
				require.NoError(t, externalaccount.BindVerifiedIdentity(t.Context(), source, other, goth.User{UserID: "next"}, false))
			}
			count, err := db.GetEngine(t.Context()).Count(new(user_model.User))
			require.NoError(t, err)
			defer test.MockVariableValue(&gothic.CompleteUserAuth, func(http.ResponseWriter, *http.Request) (goth.User, error) {
				return goth.User{Provider: sourceName, UserID: "next"}, nil
			})()
			store := session.NewMockMemStore("feishu-lifecycle")
			if !tc.missingIntent {
				boundUser, boundSource := user.ID, source.ID
				if tc.wrongUser {
					boundUser = other.ID
				}
				if tc.wrongSource {
					boundSource = -1
				}
				for key, value := range map[string]any{
					"oauth_binding_user": boundUser, "oauth_binding_source": boundSource,
					"oauth_binding_time": time.Now().Add(tc.age).Unix(), "oauth_binding_replace": true,
				} {
					require.NoError(t, store.Set(key, value))
				}
			}
			ctx, resp := contexttest.MockContext(t, "/user/oauth2/"+sourceName+"/callback?code=fixture", contexttest.MockContextOption{SessionStore: store})
			ctx.SetPathParamRaw("provider", sourceName)
			if !tc.signedOut {
				contexttest.LoadUser(t, ctx, user.ID)
			}
			SignInOAuthCallback(ctx)
			require.Equal(t, http.StatusSeeOther, resp.Code)
			if tc.signedOut {
				assert.Equal(t, "/user/login", test.RedirectURL(resp))
			} else {
				assert.Equal(t, "/user/settings/security", test.RedirectURL(resp))
			}
			for _, key := range []string{"oauth_binding_user", "oauth_binding_source", "oauth_binding_time", "oauth_binding_replace"} {
				assert.Nil(t, store.Get(key))
			}
			_, oldExists, err := user_model.GetExternalLogin(t.Context(), source.ID, "old")
			require.NoError(t, err)
			assert.Equal(t, !tc.valid, oldExists)
			if tc.valid {
				assert.NotEmpty(t, ctx.Flash.SuccessMsg)
				ctx, replay := contexttest.MockContext(t, "/user/oauth2/"+sourceName+"/callback?code=fixture", contexttest.MockContextOption{SessionStore: store})
				ctx.SetPathParamRaw("provider", sourceName)
				contexttest.LoadUser(t, ctx, user.ID)
				SignInOAuthCallback(ctx)
				assert.Equal(t, http.StatusSeeOther, replay.Code)
				assert.NotEmpty(t, ctx.Flash.ErrorMsg)
			} else {
				assert.NotEmpty(t, ctx.Flash.ErrorMsg)
			}
			link, found, err := user_model.GetExternalLogin(t.Context(), source.ID, "next")
			require.NoError(t, err)
			assert.Equal(t, tc.valid || tc.foreignIdentity, found)
			if found {
				owner := user.ID
				if tc.foreignIdentity {
					owner = other.ID
				}
				assert.Equal(t, owner, link.UserID)
			}
			assert.Equal(t, user, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}))
			assert.Equal(t, other, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: other.ID}))
			afterCount, err := db.GetEngine(t.Context()).Count(new(user_model.User))
			require.NoError(t, err)
			assert.Equal(t, count, afterCount)
		})
	}
}
