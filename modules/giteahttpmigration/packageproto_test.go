// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"bytes"
	"encoding/base64"
	"io"
	"testing"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/packages/cargo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageProtocolPaths(t *testing.T) {
	npmPath, err := npmPackagePath("alice", "@scope/widget", "1.2.3", "widget-1.2.3.tgz")
	require.NoError(t, err)
	assert.Equal(t, "/api/packages/alice/npm/@scope/widget/-/1.2.3/widget-1.2.3.tgz", npmPath)

	mavenPath, err := mavenFilePath("alice", "com.example:app", "1.0.0", "app-1.0.0.jar")
	require.NoError(t, err)
	assert.Equal(t, "/api/packages/alice/maven/com/example/app/1.0.0/app-1.0.0.jar", mavenPath)
	_, err = mavenFilePath("alice", "not-coordinates", "1.0.0", "app.jar")
	require.Error(t, err)
}

func TestNpmPublishDocumentHasIntegrity(t *testing.T) {
	document, err := npmPublishDocument("widget", "1.0.0", "widget-1.0.0.tgz", []byte("tarball"))
	require.NoError(t, err)
	var parsed struct {
		Name     string `json:"name"`
		Versions map[string]struct {
			Dist struct {
				Integrity string `json:"integrity"`
			} `json:"dist"`
		} `json:"versions"`
		Attachments map[string]struct {
			Data string `json:"data"`
		} `json:"_attachments"`
	}
	require.NoError(t, json.Unmarshal(document, &parsed))
	assert.Equal(t, "widget", parsed.Name)
	assert.Greater(t, len(parsed.Versions["1.0.0"].Dist.Integrity), len("sha512-"))
	raw, err := base64.StdEncoding.DecodeString(parsed.Attachments["widget-1.0.0.tgz"].Data)
	require.NoError(t, err)
	assert.Equal(t, "tarball", string(raw))
}

func TestCargoPublishBodyRoundTrip(t *testing.T) {
	body, err := cargoPublishBody("widget", "1.0.0", []byte("crate-bytes"))
	require.NoError(t, err)
	parsed, err := cargo.ParsePackage(bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "widget", parsed.Name)
	assert.Equal(t, "1.0.0", parsed.Version)
	content, err := io.ReadAll(parsed.Content)
	require.NoError(t, err)
	assert.Equal(t, "crate-bytes", string(content))
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
