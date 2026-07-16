// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models" // register models
	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"
	_ "github.com/neko233-com/gitsss/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
