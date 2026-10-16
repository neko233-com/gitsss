// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/neko233-com/gitsss/models/organization"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/templates"
	shared_user "github.com/neko233-com/gitsss/routers/web/shared/user"
	"github.com/neko233-com/gitsss/services/context"
)

var taskMention = regexp.MustCompile(`(?:^|\s)@([\pL\pN_.-]+)`)

type taskColumn struct {
	Status string
	Title  string
	Tasks  []*organization.OrgTask
}

// RequireKanbanMember keeps shared organization tasks private even for public organizations.
func RequireKanbanMember(ctx *context.Context) {
	if ctx.Org.Organization == nil || !ctx.Org.IsMember || (ctx.Doer.IsRestricted && !ctx.Org.IsOwner) {
		ctx.NotFound(nil)
	}
}

func kanbanMembers(ctx *context.Context) (user_model.UserList, error) {
	members, _, err := organization.FindOrgMembers(ctx, &organization.FindOrgMembersOpts{
		OrgID: ctx.ContextUser.ID, Doer: ctx.Doer, IsDoerMember: true,
	})
	return members, err
}

func taskAssignee(title, explicit string, members user_model.UserList) (int64, error) {
	name := strings.TrimPrefix(strings.TrimSpace(explicit), "@")
	if name == "" {
		if matches := taskMention.FindAllStringSubmatch(title, -1); len(matches) > 0 {
			name = matches[len(matches)-1][1]
		}
	}
	if name == "" || name == "-" {
		return 0, nil
	}
	for _, member := range members {
		if strings.EqualFold(member.Name, name) {
			return member.ID, nil
		}
	}
	return 0, organization.ErrInvalidOrgTask
}

func Kanban(ctx *context.Context) {
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("RenderUserOrgHeader", err)
		return
	}
	members, err := kanbanMembers(ctx)
	if err != nil {
		ctx.ServerError("KanbanMembers", err)
		return
	}
	query, assigneeID := ctx.FormTrim("q"), ctx.FormInt64("assignee")
	searchQuery := query
	if matches := taskMention.FindAllStringSubmatch(query, -1); len(matches) > 0 {
		if id, err := taskAssignee(query, "", members); err == nil {
			assigneeID = id
			searchQuery = strings.TrimSpace(taskMention.ReplaceAllString(query, ""))
		}
	}
	tasks, total, err := organization.FindOrgTasks(ctx, ctx.ContextUser.ID, searchQuery, assigneeID)
	if err != nil {
		ctx.ServerError("FindOrgTasks", err)
		return
	}
	columns := prepareKanbanCardData(ctx, members)
	for _, task := range tasks {
		for i := range columns {
			if task.Status == columns[i].Status {
				columns[i].Tasks = append(columns[i].Tasks, task)
			}
		}
	}
	ctx.Data["Title"] = ctx.Tr("org.global_kanban")
	ctx.Data["PageIsOrgKanban"] = true
	ctx.Data["BoardQuery"] = query
	ctx.Data["BoardAssignee"] = assigneeID
	ctx.Data["BoardTotal"] = total
	ctx.Data["BoardLimited"] = total > int64(len(tasks))
	ctx.HTML(http.StatusOK, templates.TplName("org/kanban"))
}

func prepareKanbanCardData(ctx *context.Context, members user_model.UserList) []taskColumn {
	columns := []taskColumn{
		{Status: "todo", Title: ctx.Locale.TrString("org.kanban.todo")},
		{Status: "doing", Title: ctx.Locale.TrString("org.kanban.doing")},
		{Status: "done", Title: ctx.Locale.TrString("org.kanban.done")},
	}
	names := make(map[int64]string, len(members))
	for _, member := range members {
		names[member.ID] = member.Name
	}
	ctx.Data["KanbanLink"] = ctx.ContextUser.HomeLink() + "/-/kanban"
	ctx.Data["BoardColumns"] = columns
	ctx.Data["BoardMembers"] = members
	ctx.Data["BoardNames"] = names
	return columns
}

func SaveKanbanTask(ctx *context.Context) {
	link := ctx.ContextUser.HomeLink() + "/-/kanban"
	if ctx.PathParam("id") != "" && ctx.PathParamInt64("id") <= 0 {
		ctx.NotFound(nil)
		return
	}
	task := &organization.OrgTask{OrgID: ctx.ContextUser.ID, CreatorID: ctx.Doer.ID}
	if id := ctx.PathParamInt64("id"); id > 0 {
		var found bool
		var err error
		task, found, err = organization.GetOrgTask(ctx, ctx.ContextUser.ID, id)
		if err != nil {
			ctx.ServerError("GetOrgTask", err)
			return
		}
		if !found {
			ctx.NotFound(nil)
			return
		}
	}
	members, err := kanbanMembers(ctx)
	if err != nil {
		ctx.ServerError("KanbanMembers", err)
		return
	}
	task.Title, task.Content, task.Status = ctx.FormTrim("title"), ctx.FormString("content"), ctx.FormString("status")
	task.AssigneeID, err = taskAssignee(task.Title, ctx.FormString("assignee"), members)
	if err == nil {
		if task.ID == 0 {
			err = organization.CreateOrgTask(ctx, task)
		} else {
			_, err = organization.UpdateOrgTask(ctx, task)
		}
	}
	if errors.Is(err, organization.ErrInvalidOrgTask) {
		if ctx.Req.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			ctx.JSON(http.StatusBadRequest, map[string]string{"message": ctx.Locale.TrString("org.kanban.invalid")})
			return
		}
		ctx.Flash.Error(ctx.Tr("org.kanban.invalid"))
	} else if err != nil {
		ctx.ServerError("SaveOrgTask", err)
		return
	}
	if err == nil && ctx.Req.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		prepareKanbanCardData(ctx, members)
		ctx.Data["Task"] = task
		ctx.HTML(http.StatusOK, templates.TplName("org/kanban_card"))
		return
	}
	ctx.Redirect(link)
}

func MoveKanbanTask(ctx *context.Context) {
	found, err := organization.MoveOrgTask(ctx, ctx.ContextUser.ID, ctx.PathParamInt64("id"), ctx.FormString("status"))
	if errors.Is(err, organization.ErrInvalidOrgTask) {
		ctx.JSON(http.StatusBadRequest, map[string]string{"message": ctx.Locale.TrString("org.kanban.invalid")})
	} else if err != nil {
		ctx.ServerError("MoveOrgTask", err)
	} else if !found {
		ctx.NotFound(nil)
	} else {
		ctx.JSONOK()
	}
}

func DeleteKanbanTask(ctx *context.Context) {
	task, found, err := organization.GetOrgTask(ctx, ctx.ContextUser.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetOrgTask", err)
		return
	}
	if !found || (task.CreatorID != ctx.Doer.ID && !ctx.Org.IsOwner) {
		ctx.NotFound(nil)
		return
	}
	if err := organization.DeleteOrgTask(ctx, ctx.ContextUser.ID, task.ID); err != nil {
		ctx.ServerError("DeleteOrgTask", err)
		return
	}
	if ctx.Req.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		ctx.JSONOK()
		return
	}
	ctx.Redirect(ctx.ContextUser.HomeLink() + "/-/kanban")
}
