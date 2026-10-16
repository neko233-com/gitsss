// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestTagProtection(t *testing.T) {
	log.Println("== TestTagProtection ==")

	c := newTestClient()

	name := "TestTagProtection"

	repo, clean, err := prepareTagProtectionTest(t, c, name)
	if err != nil {
		return
	}
	defer clean()

	namePattern := "v*"
	pTag, resp, err := c.Repositories.CreateTagProtection(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateTagProtectionOption{
		NamePattern:        namePattern,
		WhitelistUsernames: []string{"test01"},
		WhitelistTeams:     []string{"test_team"},
	})

	assert.NoError(t, err)
	assert.EqualValues(t, 201, resp.StatusCode)
	assert.EqualValues(t, namePattern, pTag.NamePattern)

	pTags, resp, err := c.Repositories.ListTagProtection(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoTagProtectionsOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag, pTags[0])

	pTag, resp, err = c.Repositories.GetTagProtection(t.Context(), repo.Owner.UserName, repo.Name, pTag.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag.NamePattern, namePattern)

	newNamePattern := "v*-rc"
	pTag, resp, err = c.Repositories.EditTagProtection(t.Context(), repo.Owner.UserName, repo.Name, pTag.ID, gitea.EditTagProtectionOption{
		NamePattern:        &newNamePattern,
		WhitelistUsernames: []string{"test02"},
		WhitelistTeams:     nil,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag.NamePattern, newNamePattern)

	resp, err = c.Repositories.DeleteTagProtection(t.Context(), repo.Owner.UserName, repo.Name, pTag.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)
}

func prepareTagProtectionTest(t *testing.T, c *gitea.Client, name string) (*gitea.Repository, func(), error) {
	clean, repo, err := createTestOrgRepo(t, c, name)
	if err != nil {
		return nil, nil, err
	}

	if _, err = createTestOrgTeams(t, c, repo.Owner.UserName, "test_team", gitea.AccessModeWrite, []gitea.RepoUnitType{gitea.RepoUnitCode}); err != nil {
		clean()
		return nil, nil, err
	}

	if _, err = c.Repositories.AddRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "test_team"); err != nil {
		clean()
		return nil, nil, err
	}

	tUser1 := createTestUser(t, "test01", c)
	tUser2 := createTestUser(t, "test02", c)
	write := gitea.AccessModeWrite

	_, err = c.Repositories.AddCollaborator(t.Context(), repo.Owner.UserName, repo.Name, tUser1.UserName, gitea.AddCollaboratorOption{
		Permission: &write,
	})
	if err != nil {
		clean()
		return nil, nil, err
	}

	_, err = c.Repositories.AddCollaborator(t.Context(), repo.Owner.UserName, repo.Name, tUser2.UserName, gitea.AddCollaboratorOption{
		Permission: &write,
	})
	if err != nil {
		clean()
		return nil, nil, err
	}

	return repo, clean, nil
}
