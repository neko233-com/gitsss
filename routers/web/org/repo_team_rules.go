// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/neko233-com/gitsss/models/organization"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/util"
	shared_user "github.com/neko233-com/gitsss/routers/web/shared/user"
	"github.com/neko233-com/gitsss/services/context"
	repo_service "github.com/neko233-com/gitsss/services/repository"
)

func repoTeamRulesPage(ctx *context.Context, status int, draft *organization.RepoTeamRule, preview bool) {
	state, err := organization.LoadRepoTeamRules(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("LoadRepoTeamRules", err)
		return
	}
	teams, err := organization.FindOrgTeams(ctx, ctx.Org.Organization.ID)
	if err != nil {
		ctx.ServerError("FindOrgTeams", err)
		return
	}
	labels := map[int64]string{}
	var selectable []*organization.Team
	for _, team := range teams {
		labels[team.ID] = team.DisplayLabel()
		if !team.IsOwnerTeam() && !team.IncludesAllRepositories {
			selectable = append(selectable, team)
		}
	}
	if draft == nil {
		draft = &organization.RepoTeamRule{Enabled: true}
		for _, rule := range state.Rules {
			if rule.ID == ctx.FormString("edit") {
				*draft = rule
				break
			}
		}
	}
	if preview {
		repos, err := repo_model.GetOrgRepositories(ctx, ctx.Org.Organization.ID)
		if err != nil {
			ctx.ServerError("GetOrgRepositories", err)
			return
		}
		var matches []*repo_model.Repository
		candidate := *draft
		candidate.Enabled = true
		matcher := &organization.RepoTeamRules{Rules: []organization.RepoTeamRule{candidate}}
		for _, repo := range repos {
			ids, err := matcher.Match(repo.Name)
			if err == nil && len(ids) > 0 {
				matches = append(matches, repo)
			}
		}
		ctx.Data["PreviewRepos"], ctx.Data["ShowPreview"] = matches, true
	}
	selected := map[int64]bool{}
	for _, id := range draft.TeamIDs {
		selected[id] = true
	}
	ctx.Data["Title"] = ctx.Tr("org.team_rules.title")
	ctx.Data["PageIsOrgSettings"], ctx.Data["PageIsSettingsTeamRules"] = true, true
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["RuleState"], ctx.Data["RuleTeams"], ctx.Data["TeamLabels"] = state, selectable, labels
	ctx.Data["DraftRule"], ctx.Data["SelectedRuleTeams"] = draft, selected
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}
	ctx.HTML(status, "org/settings/team_rules")
}

// RepoTeamRules is restricted to organization owners by the route assignment.
func RepoTeamRules(ctx *context.Context) {
	repoTeamRulesPage(ctx, http.StatusOK, nil, false)
}

func RepoTeamRulesPost(ctx *context.Context) {
	orgID := ctx.Org.Organization.ID
	state, err := organization.LoadRepoTeamRules(ctx, orgID)
	if err != nil {
		ctx.ServerError("LoadRepoTeamRules", err)
		return
	}
	revision, err := strconv.ParseInt(ctx.FormString("revision"), 10, 64)
	if err != nil || revision != state.Revision {
		ctx.Status(http.StatusConflict)
		return
	}
	id, action := ctx.FormString("id"), ctx.FormString("action")
	index := slices.IndexFunc(state.Rules, func(rule organization.RepoTeamRule) bool { return rule.ID == id })
	if id != "" && index < 0 {
		ctx.Status(http.StatusNotFound)
		return
	}
	switch action {
	case "apply":
		var count int
		count, err = repo_service.ApplyRepoTeamRule(ctx, orgID, revision, id)
		if err == nil {
			ctx.Flash.Success(ctx.Tr("org.team_rules.applied", count))
		}
	case "delete":
		if index < 0 {
			ctx.Status(http.StatusBadRequest)
			return
		}
		state.Rules = slices.Delete(state.Rules, index, index+1)
		err = organization.SaveRepoTeamRules(ctx, orgID, revision, state.Rules)
	case "save", "preview":
		draft := organization.RepoTeamRule{ID: id, Pattern: strings.TrimSpace(ctx.FormString("pattern")), Enabled: ctx.FormBool("enabled")}
		for _, value := range ctx.FormStrings("team_id") {
			teamID, parseErr := strconv.ParseInt(value, 10, 64)
			if parseErr != nil {
				ctx.Status(http.StatusBadRequest)
				return
			}
			draft.TeamIDs = append(draft.TeamIDs, teamID)
		}
		if draft.ID == "" {
			draft.ID = util.CryptoRandomString(32)
		}
		teams, loadErr := organization.FindOrgTeams(ctx, orgID)
		if loadErr != nil {
			ctx.ServerError("FindOrgTeams", loadErr)
			return
		}
		if organization.ValidateRepoTeamRules([]organization.RepoTeamRule{draft}, teams) != nil {
			ctx.Flash.Error(ctx.Tr("org.team_rules.invalid"))
			if id == "" {
				draft.ID = ""
			}
			repoTeamRulesPage(ctx, http.StatusBadRequest, &draft, false)
			return
		}
		if action == "preview" {
			if id == "" {
				draft.ID = ""
			}
			repoTeamRulesPage(ctx, http.StatusOK, &draft, true)
			return
		}
		if index >= 0 {
			state.Rules[index] = draft
		} else {
			state.Rules = append(state.Rules, draft)
		}
		err = organization.SaveRepoTeamRules(ctx, orgID, revision, state.Rules)
	default:
		ctx.Status(http.StatusBadRequest)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, organization.ErrRepoTeamRuleConflict):
			ctx.Status(http.StatusConflict)
		case errors.Is(err, organization.ErrRepoTeamRuleInvalid):
			ctx.Status(http.StatusBadRequest)
		default:
			ctx.ServerError("SaveRepoTeamRules", err)
		}
		return
	}
	ctx.Redirect(ctx.Org.OrgLink + "/settings/team-rules")
}
