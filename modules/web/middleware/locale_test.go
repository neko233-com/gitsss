// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAcceptLanguage(t *testing.T) {
	// a normal header is parsed and its leading language preserved
	tags := parseAcceptLanguage("de-DE,de;q=0.9,en;q=0.8")
	assert.NotEmpty(t, tags)
	assert.Equal(t, "de-DE", tags[0].String())

	// an oversized "_"-separated header would drive ParseAcceptLanguage into its
	// quadratic-time path (the built-in guard only counts "-"); the length bound
	// keeps the input passed to the parser small so it cannot be used for a DoS.
	malicious := strings.Repeat("_aaaaaaaaa", 1<<16) // ~640 KiB, zero "-" characters
	assert.Greater(t, len(malicious), maxAcceptLanguageLen)
	tags = parseAcceptLanguage(malicious)
	// no panic / hang, and nothing meaningful is parsed out of the garbage
	assert.Empty(t, tags)
}

func TestAutomaticLocaleAndExplicitPreference(t *testing.T) {
	defer test.MockVariableValue(&setting.Langs, []string{"en-US", "zh-CN", "fr-FR"})()
	defer test.MockVariableValue(&setting.Names, []string{"English", "简体中文", "Français"})()
	defer test.MockVariableValue(&setting.StaticRootPath, filepath.Join("..", "..", ".."))()
	defer test.MockVariableValue(&setting.IsProd, true)()
	translation.InitLocales(t.Context())
	for _, c := range []struct{ query, cookie, header, expected, saved string }{
		{"", "", "zh-CN,zh;q=0.9,en;q=0.8", "zh-CN", ""},
		{"", "", "fr-FR", "fr-FR", ""},
		{"", "auto", "zh-CN", "zh-CN", ""},
		{"", "auto", "fr-FR", "fr-FR", ""},
		{"", "en-US", "zh-CN", "en-US", ""},
		{"?lang=auto", "en-US", "zh-CN", "zh-CN", "auto"},
		{"?lang=zh-CN", "en-US", "en-US", "zh-CN", "zh-CN"},
		{"?lang=invalid", "", "zh-CN", "zh-CN", ""},
	} {
		r := httptest.NewRequest(http.MethodGet, "/"+c.query, nil)
		r.Header.Set("Accept-Language", c.header)
		if c.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "lang", Value: c.cookie})
		}
		w := httptest.NewRecorder()
		require.Equal(t, c.expected, Locale(w, r).Language(), "%+v", c)
		if c.saved != "" {
			require.Len(t, w.Result().Cookies(), 1)
			require.Equal(t, c.saved, w.Result().Cookies()[0].Value)
		}
	}
	t.Run("parallel instance language cookie", func(t *testing.T) {
		defer test.MockVariableValue(&setting.LocaleCookieName, "gitsss_language_5030")()
		r := httptest.NewRequest(http.MethodGet, "/?lang=auto", nil)
		r.Header.Set("Accept-Language", "zh-CN")
		r.AddCookie(&http.Cookie{Name: "lang", Value: "en-US"})
		w := httptest.NewRecorder()
		require.Equal(t, "zh-CN", Locale(w, r).Language())
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, "gitsss_language_5030", cookies[0].Name)
		require.Equal(t, "auto", cookies[0].Value)
	})
}
