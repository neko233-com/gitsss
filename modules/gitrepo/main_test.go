// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/setting"
)

func TestMain(m *testing.M) {
	// resolve repository path relative to the test directory
	setting.SetupGiteaTestEnv()
	giteaRoot := setting.GetGiteaTestSourceRoot()
	repoPath = func(repo Repository) string {
		if filepath.IsAbs(repo.RelativePath()) {
			return repo.RelativePath() // for testing purpose only
		}
		return filepath.Join(giteaRoot, "modules/git/tests/repos", repo.RelativePath())
	}
	git.RunGitTests(m)
}
