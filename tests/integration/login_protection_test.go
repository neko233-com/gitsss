// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSigninLoginProtection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.LoginProtectionEnabled, true)()
	session := emptyTestSession(t)
	for range 5 {
		req := NewRequestWithValues(t, http.MethodPost, "/user/login", map[string]string{"user_name": "user2", "password": "incorrect-protection-test"})
		req.RemoteAddr = "198.51.100.207:8000"
		session.MakeRequest(t, req, http.StatusOK)
	}
	// A correct password from a first-time client needs a challenge rather than an account unlock.
	req := NewRequestWithValues(t, http.MethodPost, "/user/login", map[string]string{"user_name": "user2", "password": userPassword})
	req.RemoteAddr = "198.51.100.207:8001"
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "login-verification-hint")
	assert.NotContains(t, resp.Body.String(), `value="`+userPassword+`"`)
	// Another user on the same NAT can still authenticate normally.
	colleague := emptyTestSession(t)
	req = NewRequestWithValues(t, http.MethodPost, "/user/login", map[string]string{"user_name": "user1", "password": userPassword})
	req.RemoteAddr = "198.51.100.207:8002"
	colleague.MakeRequest(t, req, http.StatusSeeOther)
	// Seed the real provider's isolated cache, then exercise the complete HTTP handler.
	captcha := context.GetImageCaptcha()
	const id = "http-login-protection-fixture"
	require.NoError(t, captcha.Store.Put(captcha.CachePrefix+id, string([]byte{1, 2, 3, 4, 5, 6}), 60))
	req = NewRequestWithValues(t, http.MethodPost, "/user/login", map[string]string{"user_name": "user2", "password": userPassword, "captcha_id": id, "captcha": "123456"})
	req.RemoteAddr = "198.51.100.207:8003"
	session.MakeRequest(t, req, http.StatusSeeOther)
	assert.False(t, captcha.Store.IsExist(captcha.CachePrefix+id))
}

func TestHTTPBasicLoginProtection(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.LoginProtectionEnabled, true)()
	for range 10 {
		req := NewRequest(t, http.MethodGet, "/api/v1/user").AddBasicAuth("unknown-basic-protection", "incorrect")
		req.RemoteAddr = "198.51.100.208:8000"
		MakeRequest(t, req, http.StatusUnauthorized)
	}
	req := NewRequest(t, http.MethodGet, "/api/v1/user").AddBasicAuth("unknown-basic-protection", "another-guess")
	req.RemoteAddr = "198.51.100.208:8001"
	resp := MakeRequest(t, req, http.StatusTooManyRequests)
	assert.NotEmpty(t, resp.Header().Get("Retry-After"))
	req = NewRequest(t, http.MethodGet, "/api/v1/user").AddBasicAuth("user4", userPassword)
	req.RemoteAddr = "198.51.100.208:8002"
	MakeRequest(t, req, http.StatusOK)
}
