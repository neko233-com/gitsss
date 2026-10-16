// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

// standard func to create a init repo for test routines
func createTestRepo(t *testing.T, name string, c *gitea.Client) (*gitea.Repository, error) {
	user, _, uErr := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, uErr)
	repo, _, err := c.Repositories.GetRepo(t.Context(), user.UserName, name)
	// We need to check that the received repo is not a
	// redirected one, it could be the case that gitea redirect us
	// to a new repo(because it e.g. was transferred or renamed).
	if err == nil && repo.Owner.UserName == user.UserName {
		_, _ = c.Repositories.DeleteRepo(t.Context(), user.UserName, name)
	}

	repo, _, err = c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
		Name:        name,
		Description: "A test Repo: " + name,
		AutoInit:    true,
		Gitignores:  "C,C++",
		License:     "MIT",
		Readme:      "Default",
		IssueLabels: "Default",
		Private:     false,
	})
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	return repo, err
}

func createTestUser(t *testing.T, username string, client *gitea.Client) *gitea.User {
	user, _, _ := client.Users.GetUserInfo(t.Context(), username)
	if user.ID != 0 {
		return user
	}
	user, _, err := client.Admin.CreateUser(t.Context(), gitea.CreateUserOption{Username: username, Password: username + "!1234", Email: username + "@gitea.io", MustChangePassword: gitea.OptionalBool(false), SendNotify: false})
	assert.NoError(t, err)
	return user
}
