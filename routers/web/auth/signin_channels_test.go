// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"bytes"
	"html/template"
	"os"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/translation"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"

	"github.com/stretchr/testify/require"
)

type channelProvider struct {
	oauth2.Provider
	kind, source string
}

func (p channelProvider) Name() string        { return p.kind }
func (p channelProvider) DisplayName() string { return p.source }

func TestSignInChannelsPreserveConfiguredSources(t *testing.T) {
	old := setting.AppSubURL
	setting.AppSubURL = "/git"
	t.Cleanup(func() { setting.AppSubURL = old })
	channels := buildSignInChannels([]oauth2.Provider{channelProvider{kind: "feishu", source: "feishu"}, channelProvider{kind: "feishu", source: "Team identity"}, channelProvider{kind: "openidConnect", source: "Company SSO"}})
	require.Len(t, channels, 7)
	require.Equal(t, "gitea-feishu", channels[0].Icon)
	require.Equal(t, "/git/user/oauth2/feishu", channels[0].URL)
	require.Empty(t, channels[0].SourceName)
	require.Equal(t, "/git/user/oauth2/Team%20identity", channels[1].URL)
	require.Equal(t, "Team identity", channels[1].SourceName)
	for _, channel := range channels[2:6] {
		require.Empty(t, channel.URL)
		require.Nil(t, channel.Provider)
	}
	require.Equal(t, "/git/user/oauth2/Company%20SSO", channels[6].URL)
}

func TestUnavailableSignInChannelsHaveNoAuthorizationLinks(t *testing.T) {
	source, err := os.ReadFile("../../../templates/user/auth/external_auth_methods.tmpl")
	require.NoError(t, err)
	tmpl, err := template.New("channels").Funcs(template.FuncMap{
		"ctx":        func() any { return struct{ Locale translation.MockLocale }{} },
		"svg":        func(...any) template.HTML { return "" },
		"AppSubUrl":  func() string { return "" },
		"PathEscape": func(value string) string { return value },
	}).Parse(string(source))
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, tmpl.Execute(&output, map[string]any{"SignInChannels": buildSignInChannels(nil)}))
	require.NotContains(t, output.String(), "href=")
	require.Contains(t, output.String(), `data-channel="feishu" disabled`)
	require.Contains(t, output.String(), `aria-describedby="auth-channel-help"`)
	require.Contains(t, output.String(), "auth.ui.channel_unavailable")
}
