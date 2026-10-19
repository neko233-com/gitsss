// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/markbates/goth/providers/lark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type feishuProfileTransport string

func (profile feishuProfileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := string(profile)
	switch req.URL.Path {
	case "/open-apis/auth/v3/app_access_token/internal/":
		body = `{"code":0,"app_access_token":"fixture-app-token","expire":7200}`
	case "/open-apis/authen/v1/oidc/access_token":
		body = `{"code":0,"data":{"access_token":"fixture-token","expires_in":7200}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func TestFeishuBasicIdentityWithoutEmployeePermission(t *testing.T) {
	for _, tc := range []struct {
		name, body, expected string
		invalid              bool
	}{
		{"basic", `{"code":0,"data":{"open_id":"ou-fixture","name":"Fixture"}}`, "ou-fixture", false},
		{"legacy", `{"code":0,"data":{"open_id":"ou-fixture","user_id":"employee-fixture"}}`, "employee-fixture", false},
		{"missing", `{"code":0,"data":{"name":"Fixture"}}`, "", true},
		{"wrong-type", `{"code":0,"data":{"open_id":123}}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := gothProviders["feishu"].CreateGothProvider("feishu", "https://git.example.com/callback", &Source{ClientID: "fixture", ClientSecret: "fixture"})
			require.NoError(t, err)
			p := provider.(*lark.Provider)
			p.HTTPClient = &http.Client{Transport: feishuProfileTransport(tc.body)}
			sess := &lark.Session{}
			_, err = sess.Authorize(provider, url.Values{"code": {"fixture-code"}})
			require.NoError(t, err)
			user, err := p.FetchUser(sess)
			if err == nil {
				user, err = feishuUserIdentity(user)
			}
			if tc.invalid {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "fixture-token")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, user.UserID)
			}
		})
	}
}
