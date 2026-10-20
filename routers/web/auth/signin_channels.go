// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/url"
	"strings"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/auth/source/oauth2"
)

type signInChannel struct {
	Kind, LabelKey, Icon, SourceName, URL string
	Provider                              oauth2.Provider
}

// Only configured active providers get URLs; placeholders cannot start OAuth.
func buildSignInChannels(providers []oauth2.Provider) []signInChannel {
	catalog := []signInChannel{
		{Kind: "feishu", LabelKey: "auth.ui.channel_feishu", Icon: "gitea-feishu"},
		{Kind: "gplus", LabelKey: "auth.ui.channel_google", Icon: "gitea-google"},
		{Kind: "wechat", LabelKey: "auth.ui.channel_wechat", Icon: "gitea-wechat"},
		{Kind: "discord", LabelKey: "auth.ui.channel_discord", Icon: "gitea-discord"},
		{Kind: "github", LabelKey: "auth.ui.channel_github", Icon: "octicon-mark-github"},
	}
	var result []signInChannel
	defaultNames := map[string]string{"feishu": "飞书", "gplus": "google", "wechat": "微信"}
	known := make(map[string]bool, len(catalog))
	for _, entry := range catalog {
		known[entry.Kind] = true
		found := false
		for _, provider := range providers {
			if provider.Name() != entry.Kind {
				continue
			}
			item := entry
			item.Provider = provider
			item.URL = setting.AppSubURL + "/user/oauth2/" + url.PathEscape(provider.DisplayName())
			name := strings.ToLower(provider.DisplayName())
			if name != entry.Kind && name != defaultNames[entry.Kind] {
				item.SourceName = provider.DisplayName()
			}
			result = append(result, item)
			found = true
		}
		if !found {
			result = append(result, entry)
		}
	}
	for _, provider := range providers {
		if !known[provider.Name()] {
			result = append(result, signInChannel{Kind: provider.Name(), SourceName: provider.DisplayName(), URL: setting.AppSubURL + "/user/oauth2/" + url.PathEscape(provider.DisplayName()), Provider: provider})
		}
	}
	return result
}
