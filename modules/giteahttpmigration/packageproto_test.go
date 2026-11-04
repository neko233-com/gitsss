// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportedPackageProtocols(t *testing.T) {
	for _, kind := range []string{"go", "npm", "maven", "pypi", "cargo", "composer", "nuget"} {
		assert.False(t, packageProtocolSupported(kind), kind)
	}
	for _, kind := range []string{"generic", "container", "helm", "debian", "alpine", "rubygems"} {
		assert.True(t, packageProtocolSupported(kind), kind)
	}
}

func TestDebianAndAlpineCoordinates(t *testing.T) {
	dist, component, arch, err := debianCoordinates(PackageCopy{Name: "gitsss", Version: "1.2.0"}, PackageFile{
		Name: "gitsss_1.2.0_amd64.deb", CompositeKey: "bookworm|main",
	})
	require.NoError(t, err)
	assert.Equal(t, "bookworm", dist)
	assert.Equal(t, "main", component)
	assert.Equal(t, "amd64", arch)

	branch, repository, architecture, err := alpineCoordinates(PackageFile{Name: "gitsss.apk", CompositeKey: "v3.20|community|x86_64"})
	require.NoError(t, err)
	assert.Equal(t, "v3.20", branch)
	assert.Equal(t, "community", repository)
	assert.Equal(t, "x86_64", architecture)
	_, _, _, err = debianCoordinates(PackageCopy{Name: "gitsss", Version: "1.2.0"}, PackageFile{Name: "gitsss_1.2.0_amd64.deb"})
	require.Error(t, err)
}

func TestContainerDigests(t *testing.T) {
	digests, err := containerDigests([]byte(`{
		"config": {"digest": "sha256:aaa"},
		"layers": [{"digest": "sha256:bbb"}, {"digest": "sha256:aaa"}]
	}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"sha256:aaa", "sha256:bbb"}, digests)
}
