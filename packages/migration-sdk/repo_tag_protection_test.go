// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

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
	pTag, resp, err := c.CreateTagProtection(repo.Owner.UserName, repo.Name, CreateTagProtectionOption{
		NamePattern:        namePattern,
		WhitelistUsernames: []string{"test01"},
		WhitelistTeams:     []string{"test_team"},
	})

	assert.NoError(t, err)
	assert.EqualValues(t, 201, resp.StatusCode)
	assert.EqualValues(t, namePattern, pTag.NamePattern)

	pTags, resp, err := c.ListTagProtection(repo.Owner.UserName, repo.Name, ListRepoTagProtectionsOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag, pTags[0])

	pTag, resp, err = c.GetTagProtection(repo.Owner.UserName, repo.Name, pTag.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag.NamePattern, namePattern)

	newNamePattern := "v*-rc"
	pTag, resp, err = c.EditTagProtection(repo.Owner.UserName, repo.Name, pTag.ID, EditTagProtectionOption{
		NamePattern:        &newNamePattern,
		WhitelistUsernames: []string{"test02"},
		WhitelistTeams:     nil,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 200, resp.StatusCode)
	assert.EqualValues(t, pTag.NamePattern, newNamePattern)

	resp, err = c.DeleteTagProtection(repo.Owner.UserName, repo.Name, pTag.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)
}

func prepareTagProtectionTest(t *testing.T, c *Client, name string) (*Repository, func(), error) {
	clean, repo, err := createTestOrgRepo(t, c, name)
	if err != nil {
		return nil, nil, err
	}

	if _, err = createTestOrgTeams(t, c, repo.Owner.UserName, "test_team", AccessModeWrite, []RepoUnitType{RepoUnitCode}); err != nil {
		clean()
		return nil, nil, err
	}

	if _, err = c.AddRepoTeam(repo.Owner.UserName, repo.Name, "test_team"); err != nil {
		clean()
		return nil, nil, err
	}

	tUser1 := createTestUser(t, "test01", c)
	tUser2 := createTestUser(t, "test02", c)
	write := AccessModeWrite

	_, err = c.AddCollaborator(repo.Owner.UserName, repo.Name, tUser1.UserName, AddCollaboratorOption{
		Permission: &write,
	})
	if err != nil {
		clean()
		return nil, nil, err
	}

	_, err = c.AddCollaborator(repo.Owner.UserName, repo.Name, tUser2.UserName, AddCollaboratorOption{
		Permission: &write,
	})
	if err != nil {
		clean()
		return nil, nil, err
	}

	return repo, clean, nil
}
