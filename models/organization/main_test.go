// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"
	_ "github.com/neko233-com/gitsss/models/organization"
	_ "github.com/neko233-com/gitsss/models/repo"
	_ "github.com/neko233-com/gitsss/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
