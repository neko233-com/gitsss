// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoTeamRulesCreationAndExisting(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	rules := []organization.RepoTeamRule{{ID: "servers", Pattern: "^server-", TeamIDs: []int64{2, 7}, Enabled: true}}
	require.NoError(t, organization.SaveRepoTeamRules(t.Context(), 3, -1, rules))
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo, err := CreateRepositoryDirectly(t.Context(), doer, owner, CreateRepoOptions{Name: "server-new-rule", IsPrivate: true}, true)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, DeleteRepositoryDirectly(t.Context(), repo.ID)) })
	for _, teamID := range []int64{2, 7} {
		assert.True(t, organization.HasTeamRepo(t.Context(), 3, teamID, repo.ID))
	}
	count, err := ApplyRepoTeamRule(t.Context(), 3, 0, "servers")
	require.NoError(t, err)
	assert.Zero(t, count)
	rules[0].Pattern = "^repo3$"
	require.NoError(t, organization.SaveRepoTeamRules(t.Context(), 3, 0, rules))
	count, err = ApplyRepoTeamRule(t.Context(), 3, 1, "servers")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	count, err = ApplyRepoTeamRule(t.Context(), 3, 1, "servers")
	require.NoError(t, err)
	assert.Zero(t, count)
	assert.True(t, organization.HasTeamRepo(t.Context(), 3, 2, 3))
	assert.True(t, organization.HasTeamRepo(t.Context(), 3, 2, repo.ID))
	require.NoError(t, organization.SaveRepoTeamRules(t.Context(), 3, 1, nil))
	assert.True(t, organization.HasTeamRepo(t.Context(), 3, 2, repo.ID))
	_, err = ApplyRepoTeamRule(t.Context(), 3, 1, "servers")
	assert.ErrorIs(t, err, organization.ErrRepoTeamRuleConflict)
	unittest.CheckConsistencyFor(t, &organization.Team{ID: 2}, &repo_model.Repository{ID: repo.ID})
}
