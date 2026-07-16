// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models" // register table model
	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"
	_ "github.com/neko233-com/gitsss/models/perm/access" // register table model
	_ "github.com/neko233-com/gitsss/models/repo"        // register table model
	_ "github.com/neko233-com/gitsss/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
