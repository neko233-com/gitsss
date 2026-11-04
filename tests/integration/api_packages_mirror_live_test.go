// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/require"
)

// TestPackageMirrorLiveAcceptance is an opt-in fixture server for real package clients and browser QA.
func TestPackageMirrorLiveAcceptance(t *testing.T) {
	directory := os.Getenv("GITSSS_PACKAGE_MIRROR_ACCEPTANCE_DIR")
	if directory == "" {
		t.Skip("set GITSSS_PACKAGE_MIRROR_ACCEPTANCE_DIR for manual central-registry acceptance")
	}
	require.True(t, filepath.IsAbs(directory))
	require.NoError(t, os.MkdirAll(directory, 0o700))
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.PackageMirror)()
	defer test.MockVariableValue(&setting.AppURL)()
	setting.PackageMirror.Enabled = true
	if upstream := os.Getenv("GITSSS_PACKAGE_MIRROR_MAVEN_URL"); upstream != "" {
		setting.PackageMirror.MavenURL = upstream
	}
	require.NoError(t, os.WriteFile(filepath.Join(directory, "limits.txt"), []byte(fmt.Sprintf("MaxSize=%d MaxFileSize=%d TTL=%s", setting.PackageMirror.MaxSize, setting.PackageMirror.MaxFileSize, setting.PackageMirror.MetadataTTL)), 0o600))
	server := httptest.NewServer(testWebRoutes)
	defer func() {
		server.CloseClientConnections()
		server.Close()
	}()
	setting.AppURL = server.URL + "/"
	if host := os.Getenv("GITSSS_PACKAGE_MIRROR_PUBLIC_HOST"); host != "" {
		u, err := url.Parse(server.URL)
		require.NoError(t, err)
		setting.AppURL = "http://" + host + ":" + u.Port() + "/"
	}
	require.NoError(t, os.WriteFile(filepath.Join(directory, "url.txt"), []byte(server.URL), 0o600))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.Context().Done():
			return
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(directory, "stop")); err == nil {
				return
			}
		}
	}
}
