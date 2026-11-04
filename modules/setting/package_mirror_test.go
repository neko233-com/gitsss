// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageMirrorSettings(t *testing.T) {
	previous := PackageMirror
	t.Cleanup(func() { PackageMirror = previous })
	cfg, err := NewConfigProviderFromData("")
	require.NoError(t, err)
	require.NoError(t, loadPackageMirrorFrom(cfg))
	assert.False(t, PackageMirror.Enabled)
	assert.Equal(t, "https://proxy.golang.org", PackageMirror.GoURL)
	assert.Equal(t, 5*time.Minute, PackageMirror.MetadataTTL)
	cfg, err = NewConfigProviderFromData("[packages]\nENABLED = true")
	require.NoError(t, err)
	require.NoError(t, loadPackageMirrorFrom(cfg))
	assert.False(t, PackageMirror.Enabled)
	cfg, err = NewConfigProviderFromData("[packages]\nENABLED = true\n[packages.mirror]\nENABLED = true")
	require.NoError(t, err)
	require.NoError(t, loadPackageMirrorFrom(cfg))
	assert.True(t, PackageMirror.Enabled)
	cfg, err = NewConfigProviderFromData("[packages.mirror]\nGO_CLIENT_BASE_URL=https://mirror.example:3443/git\n")
	require.NoError(t, err)
	require.NoError(t, loadPackageMirrorFrom(cfg))
	assert.Equal(t, "https://mirror.example:3443/git", PackageMirror.GoClientBaseURL)
	for _, invalid := range []string{
		"MAVEN_URL = file:///etc/passwd",
		"NPM_URL = https://user:password@registry.example.org",
		"CARGO_INDEX_URL = file:///etc/passwd",
		"NUGET_URL = https://user:password@registry.example.org",
		"GO_URL = https://example.org?key=secret",
		"GO_CLIENT_BASE_URL = http://example.org",
		"METADATA_TTL = -1s",
		"MAX_FILE_SIZE = 999999999999999",
		"MAX_ENTRIES = 0",
		"PYPI_FILE_HOSTS = *.example.org",
	} {
		cfg, err := NewConfigProviderFromData("[packages.mirror]\n" + invalid)
		require.NoError(t, err)
		require.Error(t, loadPackageMirrorFrom(cfg), invalid)
	}
}
