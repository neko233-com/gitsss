// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestRepoStaring(t *testing.T) {
	log.Println("== TestRepoStaring ==")

	// init user2
	c := newTestClient()

	user1, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	userA := createTestUser(t, "stargazer_a", c)
	userB := createTestUser(t, "stargazer_b", c)

	repo, _ := createTestRepo(t, "toStar", c)
	if repo == nil {
		t.Skip()
	}

	is, _, err := c.Repositories.IsRepoStarring(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	assert.False(t, is)

	repos, _, err := c.Repositories.GetMyStarredRepos(t.Context())
	assert.NoError(t, err)
	assert.Len(t, repos, 0)

	_, err = c.Repositories.StarRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	c.SetSudo(userA.UserName)
	_, err = c.Repositories.StarRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	c.SetSudo(userB.UserName)
	_, err = c.Repositories.StarRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)

	users, _, err := c.Repositories.ListRepoStargazers(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListStargazersOptions{})
	assert.NoError(t, err)
	assert.Len(t, users, 3)
	assert.EqualValues(t, user1.UserName, users[0].UserName)

	_, err = c.Repositories.UnStarRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	_, err = c.Repositories.UnStarRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)

	c.SetSudo("")

	users, _, err = c.Repositories.ListRepoStargazers(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListStargazersOptions{})
	assert.NoError(t, err)
	assert.Len(t, users, 2)

	repos, _, err = c.Repositories.GetMyStarredRepos(t.Context())
	assert.NoError(t, err)
	assert.Len(t, repos, 1)

	reposNew, _, err := c.Repositories.GetStarredRepos(t.Context(), user1.UserName)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.EqualValues(t, repos, reposNew)
}
