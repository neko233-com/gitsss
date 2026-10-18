// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOfflineManual(t *testing.T) {
	handler := Handler("/gitsss/help", "/gitsss/help")
	for _, path := range []string{"/gitsss/help", "/gitsss/help/"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/gitsss/help/site/en/", w.Header().Get("Location"))
	}
	for _, path := range []string{"/gitsss/help/site/en/", "/gitsss/help/site/enterprise.html", "/gitsss/help/site/assets/screenshots/home.png", "/gitsss/help/features/enterprise-identities.md", "/gitsss/help/features/workspace-navigation.md", "/gitsss/help/verification/software-transfer-panel-2026-10-18.md"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, w.Code, path)
		require.NotEmpty(t, w.Body.Bytes())
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/gitsss/help/manual.go", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestOfflineManualNormalizedRouterPaths(t *testing.T) {
	handler := Handler("/gitsss/help", "/help")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/help", nil))
	require.Equal(t, "/gitsss/help/site/en/", w.Header().Get("Location"))
	for _, path := range []string{"/help/site/en/", "/help/site/en/?lang=en"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.URL.Path = "/help/site/en"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), "GitSSS Manual")
	}
}
