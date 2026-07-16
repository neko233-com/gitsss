// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package languagestats

import (
	"testing"

	"github.com/neko233-com/gitsss/modules/git"
)

func TestMain(m *testing.M) {
	git.RunGitTests(m)
}
