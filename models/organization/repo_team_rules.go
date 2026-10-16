// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	system_model "github.com/neko233-com/gitsss/models/system"
	"github.com/neko233-com/gitsss/modules/json"

	"xorm.io/builder"
)

const MaxRepoTeamRules = 32

var (
	ErrRepoTeamRuleInvalid  = errors.New("invalid repository team rule")
	ErrRepoTeamRuleConflict = errors.New("repository team rules changed")
)

// RepoTeamRule matches the repository name, independently of mutable team labels.
type RepoTeamRule struct {
	ID      string  `json:"id"`
	Pattern string  `json:"pattern"`
	TeamIDs []int64 `json:"team_ids"`
	Enabled bool    `json:"enabled"`
}

type RepoTeamRules struct {
	Rules    []RepoTeamRule `json:"rules"`
	Revision int64          `json:"-"`
}

func RepoTeamRulesKey(orgID int64) string {
	return fmt.Sprintf("org-repo-team-rules-%d", orgID)
}

func LoadRepoTeamRules(ctx context.Context, orgID int64) (*RepoTeamRules, error) {
	state := &RepoTeamRules{Rules: []RepoTeamRule{}, Revision: -1}
	row, has, err := db.Get[system_model.AppState](ctx, builder.Eq{"id": RepoTeamRulesKey(orgID)})
	if err != nil || !has {
		return state, err
	}
	if err := json.Unmarshal([]byte(row.Content), state); err != nil {
		return nil, err
	}
	state.Revision = row.Revision
	return state, nil
}

// ValidateRepoTeamRules disallows implicit owner access and cross-organization teams.
func ValidateRepoTeamRules(rules []RepoTeamRule, teams []*Team) error {
	if len(rules) > MaxRepoTeamRules {
		return ErrRepoTeamRuleInvalid
	}
	allowed := make(map[int64]bool, len(teams))
	for _, team := range teams {
		allowed[team.ID] = !team.IsOwnerTeam() && !team.IncludesAllRepositories
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		if rule.ID == "" || len(rule.ID) > 64 || seen[rule.ID] || strings.TrimSpace(rule.Pattern) == "" || len(rule.Pattern) > 512 || len(rule.TeamIDs) == 0 || len(rule.TeamIDs) > 64 {
			return ErrRepoTeamRuleInvalid
		}
		seen[rule.ID] = true
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return ErrRepoTeamRuleInvalid
		}
		seenTeams := map[int64]bool{}
		for _, id := range rule.TeamIDs {
			if !allowed[id] || seenTeams[id] {
				return ErrRepoTeamRuleInvalid
			}
			seenTeams[id] = true
		}
	}
	return nil
}

func SaveRepoTeamRules(ctx context.Context, orgID, revision int64, rules []RepoTeamRule) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		teams, err := FindOrgTeams(ctx, orgID)
		if err != nil {
			return err
		}
		if err := ValidateRepoTeamRules(rules, teams); err != nil {
			return err
		}
		content, err := json.Marshal(RepoTeamRules{Rules: rules})
		if err != nil {
			return err
		}
		key := RepoTeamRulesKey(orgID)
		if revision == -1 {
			has, err := db.Exist[system_model.AppState](ctx, builder.Eq{"id": key})
			if err != nil {
				return err
			} else if has {
				return ErrRepoTeamRuleConflict
			}
			return db.Insert(ctx, &system_model.AppState{ID: key, Content: string(content)})
		}
		count, err := db.GetEngine(ctx).Where("id=? AND revision=?", key, revision).
			Cols("content", "revision").Update(&system_model.AppState{Content: string(content), Revision: revision + 1})
		if err == nil && count != 1 {
			return ErrRepoTeamRuleConflict
		}
		return err
	})
}

// Match unions enabled rules; Go's RE2 syntax has bounded matching time.
func (s *RepoTeamRules) Match(name string) ([]int64, error) {
	var ids []int64
	for _, rule := range s.Rules {
		if !rule.Enabled {
			continue
		}
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, ErrRepoTeamRuleInvalid
		}
		if pattern.MatchString(name) {
			ids = append(ids, rule.TeamIDs...)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

// RemoveTeamFromRepoRules prevents a deleted team from leaving uneditable rules.
func RemoveTeamFromRepoRules(ctx context.Context, orgID, teamID int64) error {
	state, err := LoadRepoTeamRules(ctx, orgID)
	if err != nil {
		return err
	}
	changed := false
	rules := make([]RepoTeamRule, 0, len(state.Rules))
	for _, rule := range state.Rules {
		if slices.Contains(rule.TeamIDs, teamID) {
			changed = true
			rule.TeamIDs = slices.DeleteFunc(rule.TeamIDs, func(id int64) bool { return id == teamID })
		}
		if len(rule.TeamIDs) > 0 {
			rules = append(rules, rule)
		}
	}
	if changed {
		return SaveRepoTeamRules(ctx, orgID, state.Revision, rules)
	}
	return nil
}
