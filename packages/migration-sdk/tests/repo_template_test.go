// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestRepoFromTemplate(t *testing.T) {
	log.Println("== TestRepoFromTemplate ==")
	c := newTestClient()
	repo, err := createTestRepo(t, "TemplateRepo", c)
	assert.NoError(t, err)
	repo, _, err = c.Repositories.EditRepo(t.Context(), repo.Owner.UserName, repo.Name, gitea.EditRepoOption{Template: gitea.OptionalBool(true)})
	assert.NoError(t, err)
	_, err = c.Repositories.SetRepoTopics(t.Context(), repo.Owner.UserName, repo.Name, []string{"abc", "def", "ghi"})
	assert.NoError(t, err)

	newRepo, resp, err := c.Repositories.CreateRepoFromTemplate(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateRepoFromTemplateOption{
		Owner:       repo.Owner.UserName,
		Name:        "repoFromTemplate",
		Description: "",
		Topics:      true,
		Labels:      true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 201, resp.StatusCode)
	assert.False(t, newRepo.Template)

	labels, _, err := c.Repositories.ListRepoLabels(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListLabelsOptions{})
	assert.NoError(t, err)
	assert.Len(t, labels, 7)

	topics, _, _ := c.Repositories.ListRepoTopics(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoTopicsOptions{})
	assert.EqualValues(t, []string{"abc", "def", "ghi"}, topics)

	_, err = c.Repositories.DeleteRepo(t.Context(), repo.Owner.UserName, "repoFromTemplate")
	assert.NoError(t, err)
}
