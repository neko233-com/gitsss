// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"
	"time"

	"github.com/neko233-com/gitsss/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLDocsPaths(t *testing.T) {
	for _, value := range []string{"../index.html", "docs/../../secret", "/etc/passwd", `docs\index.html`, ".git/config", "docs/.gitmodules", "a\x00.html", "index.html?q=secret"} {
		_, err := CleanHTMLDocsPath(value)
		assert.Error(t, err, value)
	}
	for _, value := range []string{"docs/site", "index.html", "assets/style.css", ""} {
		_, err := CleanHTMLDocsPath(value)
		assert.NoError(t, err, value)
	}
	settings := &HTMLDocsSettings{Enabled: true, Ref: "--exec", Entry: "index.html"}
	assert.Error(t, settings.Validate())
	settings.Ref = "main"
	settings.Entry = "index.js"
	assert.Error(t, settings.Validate())
}

func TestHTMLDocsAccessScopeAndExpiry(t *testing.T) {
	now := time.Now()
	access := HTMLDocsAccess{UserID: 2, RepoID: 1, Commit: "abc", Revision: 3, Expires: now.Add(30 * time.Minute).Unix()}
	token, err := SignHTMLDocsAccess(access)
	require.NoError(t, err)
	decoded, err := VerifyHTMLDocsAccess(token, now)
	require.NoError(t, err)
	assert.Equal(t, access, *decoded)
	_, err = VerifyHTMLDocsAccess(token+"x", now)
	assert.Error(t, err)
	_, err = VerifyHTMLDocsAccess(token, now.Add(time.Hour))
	assert.Error(t, err)
}

func TestHTMLDocsOptimisticRevision(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	settings, err := GetHTMLDocsSettings(t.Context(), 1)
	require.NoError(t, err)
	assert.True(t, settings.Enabled)
	settings.Directory = "docs/site"
	require.NoError(t, SaveHTMLDocsSettings(t.Context(), 1, settings))
	assert.ErrorIs(t, SaveHTMLDocsSettings(t.Context(), 1, settings), ErrHTMLDocsConflict)
	current, err := GetHTMLDocsSettings(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), current.Revision)
	assert.Equal(t, "docs/site", current.Directory)
	current.Enabled = false
	require.NoError(t, SaveHTMLDocsSettings(t.Context(), 1, current))
	current, err = GetHTMLDocsSettings(t.Context(), 1)
	require.NoError(t, err)
	assert.False(t, current.Enabled)
}
