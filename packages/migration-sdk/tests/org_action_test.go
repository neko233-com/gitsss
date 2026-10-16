// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"net/http"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrgActionSecret(t *testing.T) {
	log.Println("== TestCreateOrgActionSecret ==")
	c := newTestClient()

	user := createTestUser(t, "org_action_user", c)
	c.SetSudo(user.UserName)
	newOrg, _, err := c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{Name: "ActionOrg"})
	assert.NoError(t, err)
	assert.NotNil(t, newOrg)

	// create secret
	resp, err := c.Actions.CreateOrgSecret(t.Context(), newOrg.Name, "test", gitea.CreateOrUpdateSecretOption{Data: "test", Description: "Test secret for organization"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// update secret
	resp, err = c.Actions.CreateOrgSecret(t.Context(), newOrg.Name, "test", gitea.CreateOrUpdateSecretOption{Data: "test2", Description: "Updated test secret for organization"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// list secrets
	secrets, _, err := c.Actions.ListOrgSecrets(t.Context(), newOrg.Name, gitea.ListOrgActionsSecretOption{})
	assert.NoError(t, err)
	assert.Len(t, secrets, 1)
}

func TestUpdateOrgActionVariable(t *testing.T) {
	log.Println("== TestUpdateOrgActionVariable ==")
	c := newTestClient()

	user := createTestUser(t, "org_action_update_var_user", c)
	c.SetSudo(user.UserName)
	org, _, err := c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{Name: "ActionUpdateVarOrg"})
	assert.NoError(t, err)
	assert.NotNil(t, org)

	// Create variable
	resp, err := c.Actions.CreateOrgVariable(t.Context(), org.Name, "UPDATE_VAR", gitea.CreateActionsVariableOption{
		Value: "before",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Update variable (success)
	resp, err = c.Actions.UpdateOrgVariable(t.Context(), org.Name, "UPDATE_VAR", gitea.UpdateActionsVariableOption{
		Value: "after",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Get variable and check updated value
	v, resp, err := c.Actions.GetOrgVariable(t.Context(), org.Name, "UPDATE_VAR")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "UPDATE_VAR", v.Name)
	assert.Equal(t, "after", v.Data)

	// Update variable (empty value, should error)
	_, err = c.Actions.UpdateOrgVariable(t.Context(), org.Name, "UPDATE_VAR", gitea.UpdateActionsVariableOption{
		Value: "",
	})
	assert.Error(t, err)

	// Update variable (not found)
	resp, err = c.Actions.UpdateOrgVariable(t.Context(), org.Name, "NOT_EXIST_VAR", gitea.UpdateActionsVariableOption{
		Value: "something",
	})
	assert.Error(t, err)
	if resp != nil {
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden)
	}
}

func TestOrgActionVariable(t *testing.T) {
	log.Println("== TestOrgActionVariable ==")
	c := newTestClient()

	user := createTestUser(t, "org_action_var_user", c)
	c.SetSudo(user.UserName)
	org, _, err := c.Organizations.CreateOrg(t.Context(), gitea.CreateOrgOption{Name: "ActionVarOrg"})
	assert.NoError(t, err)
	assert.NotNil(t, org)

	// Create variable (success)
	resp, err := c.Actions.CreateOrgVariable(t.Context(), org.Name, "TEST_VAR", gitea.CreateActionsVariableOption{
		Value: "initial",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Create duplicate variable (should return Conflict)
	resp, err = c.Actions.CreateOrgVariable(t.Context(), org.Name, "TEST_VAR", gitea.CreateActionsVariableOption{
		Value: "updated",
	})
	assert.Error(t, err)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	// List variables
	vars, _, err := c.Actions.ListOrgVariables(t.Context(), org.Name, gitea.ListOrgActionsVariableOption{})
	assert.NoError(t, err)
	assert.NotNil(t, vars)
	assert.True(t, len(vars) >= 1)
	found := false
	for _, v := range vars {
		if v.Name == "TEST_VAR" {
			found = true
			assert.Equal(t, "initial", v.Data)
		}
	}
	assert.True(t, found, "TEST_VAR should be in the list")

	// Get variable (success)
	v, resp, err := c.Actions.GetOrgVariable(t.Context(), org.Name, "TEST_VAR")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "TEST_VAR", v.Name)
	assert.Equal(t, "initial", v.Data)

	// Get variable (not found)
	_, resp, err = c.Actions.GetOrgVariable(t.Context(), org.Name, "NOT_EXIST")
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Create variable (invalid cases)
	cases := []struct {
		name        string
		varName     string
		opt         gitea.CreateActionsVariableOption
		expectError bool
	}{
		{
			name:        "empty name",
			varName:     "",
			opt:         gitea.CreateActionsVariableOption{Value: "v"},
			expectError: true,
		},
		{
			name:        "empty value",
			varName:     "EMPTY_VALUE",
			opt:         gitea.CreateActionsVariableOption{Value: ""},
			expectError: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.Actions.CreateOrgVariable(t.Context(), org.Name, tc.varName, tc.opt)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if resp != nil {
				assert.True(t, resp.StatusCode >= 400)
			}
		})
	}
}
