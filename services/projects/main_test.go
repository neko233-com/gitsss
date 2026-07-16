// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
