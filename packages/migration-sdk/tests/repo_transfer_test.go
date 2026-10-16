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

func TestRepoTransfer(t *testing.T) {
	log.Printf("== TestRepoTransfer ==")
	c := newTestClient()
	owner := mustCurrentUsername(t, c)

	org, _, err := c.Admin.CreateOrg(t.Context(), owner, gitea.CreateOrgOption{Name: "TransferOrg"})
	assert.NoError(t, err)
	repo, err := createTestRepo(t, "ToMove", c)
	assert.NoError(t, err)

	newRepo, _, err := c.Repositories.TransferRepo(t.Context(), owner, repo.Name, gitea.TransferRepoOption{NewOwner: org.Name})
	assert.NoError(t, err) // admin transfer repository will execute immediately but not set as pendding.
	assert.NotNil(t, newRepo)
	assert.EqualValues(t, "ToMove", newRepo.Name)

	repo, err = createTestRepo(t, "ToMove", c)
	assert.NoError(t, err)
	_, resp, err := c.Repositories.TransferRepo(t.Context(), owner, repo.Name, gitea.TransferRepoOption{NewOwner: org.Name})
	assert.EqualValues(t, 422, resp.StatusCode)
	assert.Error(t, err)

	_, err = c.Repositories.DeleteRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	_, err = c.Repositories.DeleteRepo(t.Context(), newRepo.Owner.UserName, newRepo.Name)
	assert.NoError(t, err)
	_, err = c.Organizations.DeleteOrg(t.Context(), org.Name)
	assert.NoError(t, err)
}
