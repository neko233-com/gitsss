// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeIdentityAuthorizationURLs(t *testing.T) {
	for _, name := range []string{"gplus", "feishu", "wechat"} {
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
			if name == "wechat" {
				assert.Equal(t, "snsapi_login", parsed.Query().Get("scope"))
			}
		})
	}
}
