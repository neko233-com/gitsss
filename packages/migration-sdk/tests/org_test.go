// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func createTestOrgRepo(t *testing.T, c *gitea.Client, name string) (func(), *gitea.Repository, error) {
	_, _, err := c.Organizations.GetOrg(t.Context(), name)
	if err == nil {
		_, _ = c.Organizations.DeleteOrg(t.Context(), name)
	}
	_, _, err = c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{
		Name:                      name,
		Visibility:                gitea.VisibleTypePublic,
		RepoAdminChangeTeamAccess: true,
	})
	if !assert.NoError(t, err) {
		return nil, nil, err
	}

	_, _, err = c.Repositories.GetRepo(t.Context(), name, name)
	if err == nil {
		_, _ = c.Repositories.DeleteRepo(t.Context(), name, name)
	}

	repo, _, err := c.Repositories.CreateOrgRepo(t.Context(), name, gitea.CreateRepoOption{
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

	return func() {
		_, _ = c.Repositories.DeleteRepo(t.Context(), name, name)
		_, _ = c.Organizations.DeleteOrg(t.Context(), name)
	}, repo, err
}
