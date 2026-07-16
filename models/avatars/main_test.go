// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/activities"
	_ "github.com/neko233-com/gitsss/models/perm/access"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
