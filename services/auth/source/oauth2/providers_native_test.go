// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/url"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/svg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeIdentityBrandIcons(t *testing.T) {
	for provider, icon := range map[string]string{"feishu": "gitea-feishu", "wechat": "gitea-wechat"} {
		restore := svg.MockIcon(icon)
		require.Contains(t, string(gothProviders[provider].IconHTML(24)), icon)
		restore()
	}
}

func TestNativeIdentityAuthorizationURLs(t *testing.T) {
	for _, name := range []string{"gplus", "feishu", "wechat", "discord"} {
		t.Run(name, func(t *testing.T) {
			provider, err := gothProviders[name].CreateGothProvider("Identity", "https://git.example.com/user/oauth2/Identity/callback", &Source{ClientID: "fixture-id", ClientSecret: "fixture-secret"})
			require.NoError(t, err)
			session, err := provider.BeginAuth("fixture-state")
			require.NoError(t, err)
			authURL, err := session.GetAuthURL()
			require.NoError(t, err)
			parsed, err := url.Parse(authURL)
			require.NoError(t, err)
			assert.Equal(t, "https", parsed.Scheme)
			assert.Equal(t, "fixture-state", parsed.Query().Get("state"))
			assert.Contains(t, parsed.Query().Get("redirect_uri"), "/Identity/callback")
			assert.NotContains(t, authURL, "fixture-secret")
			if name == "gplus" {
				assert.Equal(t, "select_account", parsed.Query().Get("prompt"))
			}
			if name == "wechat" {
				assert.Equal(t, "snsapi_login", parsed.Query().Get("scope"))
				assert.Equal(t, "/connect/qrconnect", parsed.Path)
			}
		})
	}
}

func TestPublicOAuthCallbackBase(t *testing.T) {
	original := setting.OAuth2Client.CallbackURLBase
	t.Cleanup(func() { setting.OAuth2Client.CallbackURLBase = original })
	for _, raw := range []string{"http://git.example.com/", "https://user:secret@git.example.com/", "https://git.example.com/?secret=x", "https://git.example.com/?", "https://git.example.com/#fragment", "//git.example.com/", ":invalid"} {
		setting.OAuth2Client.CallbackURLBase = raw
		_, err := CallbackURLBase()
		require.Error(t, err)
	}
	setting.OAuth2Client.CallbackURLBase = "https://git.example.com/sub"
	base, err := CallbackURLBase()
	require.NoError(t, err)
	assert.Equal(t, "https://git.example.com/sub/", base)
	provider, err := createProvider("Google Space", &Source{Provider: "gplus", ClientID: "fixture", ClientSecret: "fixture"})
	require.NoError(t, err)
	session, err := provider.BeginAuth("state")
	require.NoError(t, err)
	target, err := session.GetAuthURL()
	require.NoError(t, err)
	parsed, err := url.Parse(target)
	require.NoError(t, err)
	assert.Equal(t, "https://git.example.com/sub/user/oauth2/Google%20Space/callback", parsed.Query().Get("redirect_uri"))
	setting.OAuth2Client.CallbackURLBase = ""
	base, err = CallbackURLBase()
	require.NoError(t, err)
	assert.Equal(t, setting.AppURL, base)
}

func TestNativeSourceConfiguration(t *testing.T) {
	for _, name := range []string{"gplus", "feishu", "wechat", "discord"} {
		cfg := &Source{Provider: name}
		assert.False(t, cfg.IsConfigured())
		cfg.ClientID = "client"
		assert.False(t, cfg.IsConfigured())
		cfg.ClientSecret = "secret"
		assert.True(t, cfg.IsConfigured())
		cfg.ClientSecret = " "
		assert.False(t, cfg.IsConfigured())
	}
}

func TestSourceSpecificPrivateCallback(t *testing.T) {
	source := &Source{Provider: "feishu", CallbackURLBase: "http://192.168.1.10:5030/"}
	base, err := source.CallbackBaseURL()
	require.NoError(t, err)
	assert.Equal(t, source.CallbackURLBase, base)
	source.CallbackURLBase = "http://git.example.com/"
	_, err = source.CallbackBaseURL()
	require.Error(t, err)
}
