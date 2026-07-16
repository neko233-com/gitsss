// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "github.com/neko233-com/gitsss/models/issues"
	"github.com/neko233-com/gitsss/models/unittest"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"
	_ "github.com/neko233-com/gitsss/models/repo"
	_ "github.com/neko233-com/gitsss/models/user"

	"github.com/stretchr/testify/assert"
)

func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.CheckConsistencyFor(t,
		&issues_model.Issue{},
		&issues_model.PullRequest{},
		&issues_model.Milestone{},
		&issues_model.Label{},
	)
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
