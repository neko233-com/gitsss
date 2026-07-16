// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
