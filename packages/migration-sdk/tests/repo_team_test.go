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

func TestRepoTeamManagement(t *testing.T) {
	log.Println("== TestRepoTeamManagement ==")
	c := newTestClient()

	// prepare for test
	clean, repo, err := createTestOrgRepo(t, c, "RepoTeamManagement")
	if err != nil {
		return
	}
	defer clean()
	if _, err = createTestOrgTeams(t, c, repo.Owner.UserName, "Admins", gitea.AccessModeAdmin, []gitea.RepoUnitType{gitea.RepoUnitCode, gitea.RepoUnitIssues, gitea.RepoUnitPulls, gitea.RepoUnitReleases}); err != nil {
		return
	}
	if _, err = createTestOrgTeams(t, c, repo.Owner.UserName, "CodeManager", gitea.AccessModeWrite, []gitea.RepoUnitType{gitea.RepoUnitCode}); err != nil {
		return
	}
	if _, err = createTestOrgTeams(t, c, repo.Owner.UserName, "IssueManager", gitea.AccessModeWrite, []gitea.RepoUnitType{gitea.RepoUnitIssues, gitea.RepoUnitPulls}); err != nil {
		return
	}

	// test
	teams, _, err := c.Repositories.GetRepoTeams(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	if !assert.Len(t, teams, 1) {
		return
	}
	assert.EqualValues(t, gitea.AccessModeOwner, teams[0].Permission)

	team, _, err := c.Repositories.CheckRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "Admins")
	assert.NoError(t, err)
	assert.Nil(t, team)

	resp, err := c.Repositories.AddRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "Admins")
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)
	resp, err = c.Repositories.AddRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "CodeManager")
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)
	resp, err = c.Repositories.AddRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "IssueManager")
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)

	team, _, err = c.Repositories.CheckRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "Admins")
	assert.NoError(t, err)
	if assert.NotNil(t, team) {
		assert.EqualValues(t, "Admins", team.Name)
		assert.EqualValues(t, gitea.AccessModeAdmin, team.Permission)
	}

	teams, _, err = c.Repositories.GetRepoTeams(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	assert.Len(t, teams, 4)

	resp, err = c.Repositories.RemoveRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "IssueManager")
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)

	team, _, err = c.Repositories.CheckRepoTeam(t.Context(), repo.Owner.UserName, repo.Name, "IssueManager")
	assert.NoError(t, err)
	assert.Nil(t, team)
}
