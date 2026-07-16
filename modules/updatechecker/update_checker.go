// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/proxy"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/system"

	"github.com/hashicorp/go-version"
)

// CheckerState stores the remote version from the JSON endpoint
type CheckerState struct {
	LatestVersion string
}

// Name returns the name of the state item for update checker
func (r *CheckerState) Name() string {
	return "update-checker"
}

const maxManifestSize = 1024 * 1024

// GitSSSUpdateChecker records the version from an explicitly configured manifest endpoint.
func GitSSSUpdateChecker(ctx context.Context, httpEndpoint string) error {
	httpEndpoint = strings.TrimSpace(httpEndpoint)
	if httpEndpoint == "" {
		return nil
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: proxy.Proxy(),
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpEndpoint, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("update manifest returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestSize+1))
	if err != nil {
		return err
	}
	if len(body) > maxManifestSize {
		return fmt.Errorf("update manifest exceeds %d bytes", maxManifestSize)
	}

	type respType struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
	}
	respData := respType{}
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return err
	}
	if strings.TrimSpace(respData.Latest.Version) == "" {
		return errors.New("update manifest has no latest version")
	}

	return UpdateRemoteVersion(req.Context(), respData.Latest.Version)
}

// UpdateRemoteVersion updates the latest available version of gitsss.
func UpdateRemoteVersion(ctx context.Context, version string) (err error) {
	return system.AppState.Set(ctx, &CheckerState{LatestVersion: version})
}

// GetRemoteVersion returns the current remote version (or currently installed version if fail to fetch from DB)
func GetRemoteVersion(ctx context.Context) string {
	item := new(CheckerState)
	if err := system.AppState.Get(ctx, item); err != nil {
		return ""
	}
	return item.LatestVersion
}

// GetNeedUpdate returns true whether a newer version of gitsss is available.
func GetNeedUpdate(ctx context.Context) bool {
	curVer, err := version.NewVersion(setting.AppVer)
	if err != nil {
		// return false to fail silently
		return false
	}
	remoteVerStr := GetRemoteVersion(ctx)
	if remoteVerStr == "" {
		// no remote version is known
		return false
	}
	remoteVer, err := version.NewVersion(remoteVerStr)
	if err != nil {
		// return false to fail silently
		return false
	}
	return curVer.LessThan(remoteVer)
}
