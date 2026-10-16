// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestOrgTaskCRUDSearchAndOrganizationIsolation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	task := &organization.OrgTask{OrgID: 3, Title: "  Repair BuildWorld  ", Content: "SSH verification", Status: "todo", CreatorID: 2, AssigneeID: 4}
	require.NoError(t, organization.CreateOrgTask(ctx, task))
	require.Equal(t, "Repair BuildWorld", task.Title)
	tasks, total, err := organization.FindOrgTasks(ctx, 3, "buildworld", 4)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, tasks, 1)
	_, found, err := organization.GetOrgTask(ctx, 6, task.ID)
	require.NoError(t, err)
	require.False(t, found)
	found, err = organization.MoveOrgTask(ctx, 6, task.ID, "done")
	require.NoError(t, err)
	require.False(t, found)
	found, err = organization.MoveOrgTask(ctx, 3, task.ID, "doing")
	require.NoError(t, err)
	require.True(t, found)
	task.Status, task.AssigneeID = "done", 0
	found, err = organization.UpdateOrgTask(ctx, task)
	require.NoError(t, err)
	require.True(t, found)
	stored, found, err := organization.GetOrgTask(ctx, 3, task.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.EqualValues(t, 0, stored.AssigneeID)
	require.Equal(t, "done", stored.Status)
	require.NoError(t, organization.DeleteOrgTask(ctx, 6, task.ID))
	_, found, err = organization.GetOrgTask(ctx, 3, task.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, organization.DeleteOrgTask(ctx, 3, task.ID))
	_, found, err = organization.GetOrgTask(ctx, 3, task.ID)
	require.NoError(t, err)
	require.False(t, found)
}

func TestOrgTaskRejectsInvalidStateAndForeignAssignee(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	for _, task := range []*organization.OrgTask{
		{OrgID: 3, Title: "Task", Status: "todo", AssigneeID: 1},
		{OrgID: 3, Title: "Task", Status: "invalid"},
		{OrgID: 3, Title: " ", Status: "todo"},
		{OrgID: 3, Title: strings.Repeat("字", 256), Status: "todo"},
		{OrgID: 3, Title: "Task", Status: "todo", Content: strings.Repeat("a", 10001)},
	} {
		require.ErrorIs(t, organization.CreateOrgTask(t.Context(), task), organization.ErrInvalidOrgTask)
	}
}
