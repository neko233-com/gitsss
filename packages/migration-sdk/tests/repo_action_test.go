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

func TestCreateRepoActionSecret(t *testing.T) {
	log.Println("== TestCreateRepoActionSecret ==")
	c := newTestClient()

	user := createTestUser(t, "repo_action_user", c)
	c.SetSudo(user.UserName)
	newRepo, _, err := c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
		Name: "test",
	})
	assert.NoError(t, err)
	assert.NotNil(t, newRepo)

	// create secret
	resp, err := c.Actions.CreateRepoSecret(t.Context(), newRepo.Owner.UserName, newRepo.Name, "test", gitea.CreateOrUpdateSecretOption{Data: "test", Description: "Test secret for repository"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// update secret
	resp, err = c.Actions.CreateRepoSecret(t.Context(), newRepo.Owner.UserName, newRepo.Name, "test", gitea.CreateOrUpdateSecretOption{Data: "test2", Description: "Updated test secret for repository"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// list secrets
	secrets, _, err := c.Actions.ListRepoSecrets(t.Context(), newRepo.Owner.UserName, newRepo.Name, gitea.ListRepoActionsSecretOption{})
	assert.NoError(t, err)
	assert.Len(t, secrets, 1)

	// delete secret
	resp, err = c.Actions.DeleteRepoSecret(t.Context(), newRepo.Owner.UserName, newRepo.Name, "test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// confirm that secret was deleted
	secrets, _, err = c.Actions.ListRepoSecrets(t.Context(), newRepo.Owner.UserName, newRepo.Name, gitea.ListRepoActionsSecretOption{})
	assert.NoError(t, err)
	assert.Len(t, secrets, 0)
}

func TestCreateRepoActionVariable(t *testing.T) {
	log.Println("== TestCreateRepoActionVariable ==")
	c := newTestClient()

	user := createTestUser(t, "repo_action_variable_user", c)
	c.SetSudo(user.UserName)
	newRepo, _, err := c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
		Name: "test_variable",
	})
	assert.NoError(t, err)
	assert.NotNil(t, newRepo)

	// create variable
	resp, err := c.Actions.CreateRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR", "test value")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// get variable
	variable, resp, err := c.Actions.GetRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "test value", variable.Value)

	// update variable
	resp, err = c.Actions.UpdateRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR", "new updated value")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// get updated variable
	variable, resp, err = c.Actions.GetRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "new updated value", variable.Value)

	// delete variable
	resp, err = c.Actions.DeleteRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// 404 when get variable
	_, resp, _ = c.Actions.GetRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "TEST_VAR")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListRepoActionVariable(t *testing.T) {
	log.Println("== TestListRepoActionVariable ==")
	c := newTestClient()

	user := createTestUser(t, "repo_action_variable_list_user", c)
	c.SetSudo(user.UserName)
	newRepo, _, err := c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
		Name: "test_variable_list",
	})
	assert.NoError(t, err)
	assert.NotNil(t, newRepo)

	// create variable
	resp, err := c.Actions.CreateRepoVariable(t.Context(), newRepo.Owner.UserName, newRepo.Name, "MY_VAR", "my_value")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// list variables
	vars, _, err := c.Actions.ListRepoVariables(t.Context(), newRepo.Owner.UserName, newRepo.Name, gitea.ListRepoActionsVariableOption{})
	assert.NoError(t, err)
	assert.NotNil(t, vars)
	found := false
	for _, v := range vars {
		if v.Name == "MY_VAR" && v.Value == "my_value" {
			found = true
			break
		}
	}
	assert.True(t, found, "Created variable not found in list")
}
