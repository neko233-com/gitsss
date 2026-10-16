// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"context"
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func createTestOrgTeams(t *testing.T, c *gitea.Client, org, name string, accessMode gitea.AccessMode, units []gitea.RepoUnitType) (*gitea.Team, error) {
	team, _, e := c.Organizations.CreateTeam(t.Context(), org, gitea.CreateTeamOption{
		Name:                    name,
		Description:             name + "'s team desc",
		Permission:              accessMode,
		CanCreateOrgRepo:        false,
		IncludesAllRepositories: false,
		Units:                   units,
	})
	assert.NoError(t, e)
	assert.NotNil(t, team)
	return team, e
}

func TestTeamSearch(t *testing.T) {
	log.Println("== TestTeamSearch ==")
	c := newTestClient()

	orgName := "TestTeamsOrg"
	// prepare for test
	_, _, err := c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{
		Name:                      orgName,
		Visibility:                gitea.VisibleTypePublic,
		RepoAdminChangeTeamAccess: true,
	})
	cleanupCtx := context.Background()
	defer func() {
		_, _ = c.Organizations.DeleteOrg(cleanupCtx, orgName)
	}()

	assert.NoError(t, err)

	if _, err = createTestOrgTeams(t, c, orgName, "Admins", gitea.AccessModeAdmin, []gitea.RepoUnitType{gitea.RepoUnitCode, gitea.RepoUnitIssues, gitea.RepoUnitPulls, gitea.RepoUnitReleases}); err != nil {
		return
	}

	teams, _, err := c.Organizations.SearchOrgTeams(t.Context(), orgName, &gitea.SearchTeamsOptions{
		Query: "Admins",
	})
	assert.NoError(t, err)
	if assert.Len(t, teams, 1) {
		assert.Equal(t, "Admins", teams[0].Name)
	}
}
