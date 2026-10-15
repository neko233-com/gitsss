// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package updatechecker finds newer gitsss releases and verifies downloaded packages.
package updatechecker

import (
	"context"
	"errors"
	"strings"

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

// GitSSSUpdateChecker records the version from an explicitly configured manifest endpoint.
// An empty endpoint performs no network request, including when the cron task is enabled.
func GitSSSUpdateChecker(ctx context.Context, httpEndpoint string) error {
	httpEndpoint = strings.TrimSpace(httpEndpoint)
	if httpEndpoint == "" {
		return nil
	}
	report, err := Check(ctx, Options{HTTPEndpoint: httpEndpoint, CurrentVersion: setting.AppVer})
	if err != nil {
		return err
	}
	if strings.TrimSpace(report.LatestVersion) == "" {
		return errors.New("update manifest has no latest version")
	}
	return UpdateRemoteVersion(ctx, report.LatestVersion)
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
