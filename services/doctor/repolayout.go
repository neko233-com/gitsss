// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"errors"

	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/repolayout"
	"github.com/neko233-com/gitsss/modules/setting"
)

func checkRepoLayout(_ context.Context, logger log.Logger, autofix bool) error {
	if !setting.InstallLock {
		return nil
	}
	dataRoot := setting.AppDataPath
	repoRoot := setting.RepoRootPath
	lfsRoot := ""
	if setting.LFS.Storage.Type == setting.LocalStorageType {
		lfsRoot = setting.LFS.Storage.Path
	}
	manifest, err := repolayout.Read(dataRoot)
	if err != nil && autofix {
		manifest, err = repolayout.Ensure(dataRoot, repoRoot, lfsRoot)
	}
	if err != nil {
		logger.Error("Git layout check failed: %v", err)
		if errors.Is(err, repolayout.ErrFutureLayout) {
			logger.Error("Refusing to rewrite Git repositories or LFS objects for an unknown layout.")
		}
		return err
	}
	logger.Info("Git layout %d is compatible. Frozen roles: %v. Sidecar: %s", manifest.LayoutVersion, manifest.Frozen, repolayout.ManifestPath(dataRoot))
	return nil
}

func init() {
	Register(&Check{
		Title:                      "Check Git repository layout compatibility",
		Name:                       "git-layout",
		IsDefault:                  true,
		Run:                        checkRepoLayout,
		AbortIfFailed:              true,
		SkipDatabaseInitialization: true,
		Priority:                   2,
	})
}
