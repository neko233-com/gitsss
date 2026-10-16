// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestAdminOrg(t *testing.T) {
	log.Println("== TestAdminOrg ==")
	c := newTestClient()
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	orgName := "NewTestOrg"
	newOrg, _, err := c.Admin.CreateOrg(t.Context(), user.UserName, gitea.CreateOrgOption{
		Name:        orgName,
		FullName:    orgName + " FullName",
		Description: "test adminCreateOrg",
		Visibility:  gitea.VisibleTypePublic,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, newOrg)
	assert.EqualValues(t, orgName, newOrg.Name)

	orgs, _, err := c.Admin.ListOrgs(t.Context(), gitea.AdminListOrgsOptions{})
	assert.NoError(t, err)
	if assert.True(t, len(orgs) >= 1) {
		orgs = orgs[len(orgs)-1:]
		assert.EqualValues(t, newOrg.ID, orgs[0].ID)
	}

	_, err = c.Organizations.DeleteOrg(t.Context(), orgName)
	assert.NoError(t, err)
}

func TestAdminCronTasks(t *testing.T) {
	log.Println("== TestAdminCronTasks ==")
	c := newTestClient()

	tasks, _, err := c.Admin.ListCronTasks(t.Context(), gitea.ListCronTaskOptions{})
	assert.NoError(t, err)
	assert.True(t, len(tasks) > 15)
	_, err = c.Admin.RunCronTasks(t.Context(), tasks[0].Name)
	assert.NoError(t, err)
}
