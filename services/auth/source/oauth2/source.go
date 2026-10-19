// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"strings"

	"github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/modules/json"
)

// Source holds configuration for the OAuth2 login source.
type Source struct {
	auth.ConfigBase `json:"-"`

	Provider                      string
	ClientID                      string
	ClientSecret                  string
	CallbackURLBase               string
	OpenIDConnectAutoDiscoveryURL string
	CustomURLMapping              *CustomURLMapping
	IconURL                       string

	Scopes              []string
	RequiredClaimName   string
	RequiredClaimValue  string
	GroupClaimName      string
	AdminGroup          string
	GroupTeamMap        string
	GroupTeamMapRemoval bool
	RestrictedGroup     string

	SSHPublicKeyClaimName string
	FullNameClaimName     string
	ExternalIDClaim       string
}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports an OAuth2Config to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return json.Marshal(source)
}

func init() {
	auth.RegisterTypeConfig(auth.OAuth2, &Source{})
}

// IsConfigured keeps unfinished native sources out of sign-in and binding menus.
func (source *Source) IsConfigured() bool {
	if !IsNativeIdentityProvider(source.Provider) {
		return true
	}
	return strings.TrimSpace(source.ClientID) != "" && strings.TrimSpace(source.ClientSecret) != ""
}

func IsNativeIdentityProvider(name string) bool {
	switch name {
	case "gplus", "feishu", "wechat", "discord":
		return true
	}
	return false
}

func (source *Source) CallbackBaseURL() (string, error) {
	if strings.TrimSpace(source.CallbackURLBase) == "" {
		return CallbackURLBase()
	}
	return normalizeCallbackURLBase(source.CallbackURLBase)
}
