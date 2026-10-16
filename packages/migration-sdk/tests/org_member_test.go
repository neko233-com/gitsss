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

/*
// DeleteOrgMembership remove a member from an organization
func (c *Client) DeleteOrgMembership(org, user string) error {}
*/
func TestOrgMembership(t *testing.T) {
	log.Println("== TestOrgMembership ==")
	c := newTestClient()

	user := createTestUser(t, "org_mem_user", c)
	c.SetSudo(user.UserName)
	newOrg, _, err := c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{Name: "MemberOrg"})
	assert.NoError(t, err)
	assert.NotNil(t, newOrg)

	// Check func
	check, _, err := c.Organizations.CheckPublicOrgMembership(t.Context(), newOrg.Name, user.UserName)
	assert.NoError(t, err)
	assert.False(t, check)
	check, _, err = c.Organizations.CheckOrgMembership(t.Context(), newOrg.Name, user.UserName)
	assert.NoError(t, err)
	assert.True(t, check)

	perm, _, err := c.Organizations.GetOrgPermissions(t.Context(), newOrg.Name, user.UserName)
	assert.NoError(t, err)
	assert.NotNil(t, perm)
	assert.True(t, perm.IsOwner)

	_, err = c.Organizations.SetPublicOrgMembership(t.Context(), newOrg.Name, user.UserName, true)
	assert.NoError(t, err)
	check, _, err = c.Organizations.CheckPublicOrgMembership(t.Context(), newOrg.Name, user.UserName)
	assert.NoError(t, err)
	assert.True(t, check)

	u, _, err := c.Organizations.ListOrgMembership(t.Context(), newOrg.Name, gitea.ListOrgMembershipOption{})
	assert.NoError(t, err)
	assert.Len(t, u, 1)
	assert.EqualValues(t, user.UserName, u[0].UserName)
	u, _, err = c.Organizations.ListPublicOrgMembership(t.Context(), newOrg.Name, gitea.ListOrgMembershipOption{})
	assert.NoError(t, err)
	assert.Len(t, u, 1)
	assert.EqualValues(t, user.UserName, u[0].UserName)

	_, err = c.Organizations.DeleteOrgMembership(t.Context(), newOrg.Name, user.UserName)
	assert.Error(t, err)

	c.SetSudo("")
	_, err = c.Admin.DeleteUser(t.Context(), user.UserName)
	assert.Error(t, err)
	_, err = c.Organizations.DeleteOrg(t.Context(), newOrg.Name)
	assert.NoError(t, err)
	_, err = c.Admin.DeleteUser(t.Context(), user.UserName)
	assert.NoError(t, err)
}
