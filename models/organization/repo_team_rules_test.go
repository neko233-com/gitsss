// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoTeamRuleMatchAndValidation(t *testing.T) {
	teams := []*organization.Team{{ID: 2, Name: "developers"}, {ID: 7, Name: "reviewers"}, {ID: 1, Name: "Owners"}, {ID: 8, IncludesAllRepositories: true}}
	rules := []organization.RepoTeamRule{
		{ID: "a", Pattern: "^server-", TeamIDs: []int64{2, 7}, Enabled: true},
		{ID: "b", Pattern: "(?i)-go$", TeamIDs: []int64{7}, Enabled: true},
		{ID: "c", Pattern: ".*", TeamIDs: []int64{2}, Enabled: false},
	}
	require.NoError(t, organization.ValidateRepoTeamRules(rules, teams))
	state := &organization.RepoTeamRules{Rules: rules}
	ids, err := state.Match("server-game-go")
	require.NoError(t, err)
	assert.Equal(t, []int64{2, 7}, ids)
	ids, err = state.Match("client-GO")
	require.NoError(t, err)
	assert.Equal(t, []int64{7}, ids)
	ids, err = state.Match("client-web")
	require.NoError(t, err)
	assert.Empty(t, ids)
	for _, invalid := range []organization.RepoTeamRule{
		{ID: "x", Pattern: "[", TeamIDs: []int64{2}},
		{ID: "x", Pattern: "", TeamIDs: []int64{2}},
		{ID: "x", Pattern: "(a)\\1", TeamIDs: []int64{2}},
		{ID: "x", Pattern: ".*", TeamIDs: []int64{999}},
		{ID: "x", Pattern: ".*", TeamIDs: []int64{1}},
		{ID: "x", Pattern: ".*", TeamIDs: []int64{8}},
		{ID: "x", Pattern: ".*", TeamIDs: []int64{2, 2}},
	} {
		assert.ErrorIs(t, organization.ValidateRepoTeamRules([]organization.RepoTeamRule{invalid}, teams), organization.ErrRepoTeamRuleInvalid)
	}
}

func TestRepoTeamRulesStoreIsolation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	state, err := organization.LoadRepoTeamRules(t.Context(), 3)
	require.NoError(t, err)
	rules := []organization.RepoTeamRule{{ID: "server", Pattern: "^repo", TeamIDs: []int64{2}, Enabled: true}}
	require.NoError(t, organization.SaveRepoTeamRules(t.Context(), 3, state.Revision, rules))
	assert.ErrorIs(t, organization.SaveRepoTeamRules(t.Context(), 3, state.Revision, nil), organization.ErrRepoTeamRuleConflict)
	saved, err := organization.LoadRepoTeamRules(t.Context(), 3)
	require.NoError(t, err)
	assert.Equal(t, rules, saved.Rules)
	other, err := organization.LoadRepoTeamRules(t.Context(), 17)
	require.NoError(t, err)
	assert.Empty(t, other.Rules)
	assert.ErrorIs(t, organization.SaveRepoTeamRules(t.Context(), 17, other.Revision, rules), organization.ErrRepoTeamRuleInvalid)
	require.NoError(t, organization.SaveRepoTeamRules(t.Context(), 3, saved.Revision, nil))
}
