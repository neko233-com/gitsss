// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"github.com/neko233-com/gitsss/modules/git"
)

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.TagPrefix+name)
}
