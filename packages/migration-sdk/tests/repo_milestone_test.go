// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"
	"time"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestMilestones(t *testing.T) {
	log.Println("== TestMilestones ==")
	c := newTestClient()

	repo, _ := createTestRepo(t, "TestMilestones", c)
	now := time.Now()
	future := time.Unix(1896134400, 0) // 2030-02-01
	closed := "closed"
	sClosed := gitea.StateClosed

	// CreateMilestone 4x
	m1, _, err := c.Repositories.CreateMilestone(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateMilestoneOption{Title: "v1.0", Description: "First Version", Deadline: &now})
	assert.NoError(t, err)
	_, _, err = c.Repositories.CreateMilestone(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateMilestoneOption{Title: "v2.0", Description: "Second Version", Deadline: &future})
	assert.NoError(t, err)
	_, _, err = c.Repositories.CreateMilestone(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateMilestoneOption{Title: "v3.0", Description: "Third Version", Deadline: nil})
	assert.NoError(t, err)
	m4, _, err := c.Repositories.CreateMilestone(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateMilestoneOption{Title: "temp", Description: "part time milestone"})
	assert.NoError(t, err)

	// EditMilestone
	m1, _, err = c.Repositories.EditMilestone(t.Context(), repo.Owner.UserName, repo.Name, m1.ID, gitea.EditMilestoneOption{Description: &closed, State: &sClosed})
	assert.NoError(t, err)

	// DeleteMilestone
	_, err = c.Repositories.DeleteMilestone(t.Context(), repo.Owner.UserName, repo.Name, m4.ID)
	assert.NoError(t, err)

	// ListMilestones
	// default should be open, but there is a bug which will return all milestones by default from Gitea 1.22 and fixed in 1.26
	ml, _, err := c.Repositories.ListMilestones(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListMilestoneOption{})
	assert.NoError(t, err)
	assert.Len(t, ml, 2)
	ml, _, err = c.Repositories.ListMilestones(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListMilestoneOption{State: gitea.StateClosed})
	assert.NoError(t, err)
	assert.Len(t, ml, 1)
	ml, _, err = c.Repositories.ListMilestones(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListMilestoneOption{State: gitea.StateAll})
	assert.NoError(t, err)
	assert.Len(t, ml, 3)
	ml, _, err = c.Repositories.ListMilestones(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListMilestoneOption{State: gitea.StateAll, Name: "V3.0"})
	assert.NoError(t, err)
	assert.Len(t, ml, 1)
	assert.EqualValues(t, "v3.0", ml[0].Title)

	// Use the exact milestone name here so the assertion works across servers
	// regardless of whether name matching is case-sensitive or delegated to a fallback.
	m, _, err := c.Repositories.GetMilestoneByName(t.Context(), repo.Owner.UserName, repo.Name, "v3.0")
	assert.NoError(t, err)
	assert.EqualValues(t, ml[0].ID, m.ID)
	_, _, err = c.Repositories.GetMilestoneByName(t.Context(), repo.Owner.UserName, repo.Name, "NoEvidenceOfExist")
	assert.Error(t, err)

	// GetMilestone
	_, _, err = c.Repositories.GetMilestone(t.Context(), repo.Owner.UserName, repo.Name, m4.ID)
	assert.Error(t, err)
	m, _, err = c.Repositories.GetMilestone(t.Context(), repo.Owner.UserName, repo.Name, m1.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, m1, m)
	m2, _, err := c.Repositories.GetMilestoneByName(t.Context(), repo.Owner.UserName, repo.Name, m.Title)
	assert.NoError(t, err)
	assert.EqualValues(t, m, m2)
}
