// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"slices"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	repo_model "github.com/neko233-com/gitsss/models/repo"
)

// applyRepoTeamRules runs inside creation/transfer transactions, before access is exposed.
func applyRepoTeamRules(ctx context.Context, repo *repo_model.Repository, teams []*organization.Team) error {
	rules, err := organization.LoadRepoTeamRules(ctx, repo.OwnerID)
	if err != nil {
		return err
	}
	ids, err := rules.Match(repo.Name)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if !team.IsOwnerTeam() && !team.IncludesAllRepositories && slices.Contains(ids, team.ID) && !organization.HasTeamRepo(ctx, repo.OwnerID, team.ID, repo.ID) {
			if err := addRepositoryToTeam(ctx, team, repo); err != nil {
				return err
			}
		}
	}
	return nil
}

// ApplyRepoTeamRule adds matching existing repositories without removing manual grants.
func ApplyRepoTeamRule(ctx context.Context, orgID, revision int64, ruleID string) (added int, err error) {
	err = db.WithTx(ctx, func(ctx context.Context) error {
		state, err := organization.LoadRepoTeamRules(ctx, orgID)
		if err != nil {
			return err
		}
		if state.Revision != revision {
			return organization.ErrRepoTeamRuleConflict
		}
		var selected *organization.RepoTeamRule
		for _, rule := range state.Rules {
			if rule.ID == ruleID && rule.Enabled {
				selected = &rule
				break
			}
		}
		if selected == nil {
			return organization.ErrRepoTeamRuleInvalid
		}
		teams, err := organization.FindOrgTeams(ctx, orgID)
		if err != nil {
			return err
		}
		// Deleted teams are ignored. A new team with the same label has a different ID.
		repos, err := repo_model.GetOrgRepositories(ctx, orgID)
		if err != nil {
			return err
		}
		ruleState := &organization.RepoTeamRules{Rules: []organization.RepoTeamRule{*selected}}
		for _, repo := range repos {
			ids, err := ruleState.Match(repo.Name)
			if err != nil {
				return err
			}
			for _, team := range teams {
				if !team.IsOwnerTeam() && !team.IncludesAllRepositories && slices.Contains(ids, team.ID) && !organization.HasTeamRepo(ctx, orgID, team.ID, repo.ID) {
					if err := addRepositoryToTeam(ctx, team, repo); err != nil {
						return err
					}
					added++
				}
			}
		}
		return nil
	})
	return added, err
}
