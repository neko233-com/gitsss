// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/routers/web/auth"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/tests"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuth2AvatarFromPicture(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

	mockServer := createOAuth2MockProvider()
	defer mockServer.Close()
	addOAuth2Source(t, "test-oidc-avatar", oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
	})
	authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), "test-oidc-avatar")
	require.NoError(t, err)
	providerName := authSource.Cfg.(*oauth2.Source).Provider

	t.Run("AutoRegister", func(t *testing.T) {
		defer test.MockVariableValue(&setting.OAuth2Client.Username, "")()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
			return goth.User{
				Provider:  providerName,
				UserID:    "oidc-user-ua-pic",
				Email:     "oidc-user-ua-pic@example.com",
				Name:      "OIDC UA Pic",
				AvatarURL: mockServer.URL + "/avatar.png",
			}, nil
		})()

		req := NewRequest(t, "GET", "/user/oauth2/test-oidc-avatar/callback?code=XYZ&state=XYZ")
		emptyTestSession(t).MakeRequest(t, req, http.StatusSeeOther)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-ua-pic"})
		assert.True(t, user.UseCustomAvatar, "avatar must sync (requires Gitea UA)")
		assert.NotEmpty(t, user.Avatar)
	})

	t.Run("LinkAccountRegister", func(t *testing.T) {
		const newUserName = "oidc-link-register"
		defer web.RouteMockReset()
		web.RouteMock(web.MockAfterMiddlewares, func(ctx *context.Context) {
			require.NoError(t, auth.Oauth2SetLinkAccountData(ctx, auth.LinkAccountData{
				AuthSourceID: authSource.ID,
				GothUser: goth.User{
					Provider:  providerName,
					UserID:    "oidc-link-register-sub",
					Email:     "oidc-link-register-a@example.com",
					Name:      "OIDC Link Register",
					AvatarURL: mockServer.URL + "/avatar.png",
				},
			}))
		})

		req := NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
			"user_name": newUserName,
			"email":     "oidc-link-register-b@example.com",
			"password":  "AVeryStrongPassword!1",
			"retype":    "AVeryStrongPassword!1",
		})
		emptyTestSession(t).MakeRequest(t, req, http.StatusSeeOther)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: newUserName})
		require.Equal(t, auth_model.OAuth2, user.LoginType)
		assert.True(t, user.UseCustomAvatar, "register-link flow must sync avatar from `picture` claim")
		assert.NotEmpty(t, user.Avatar)
	})
}
