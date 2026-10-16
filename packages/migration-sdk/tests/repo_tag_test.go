// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"fmt"
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestTags(t *testing.T) {
	log.Println("== TestTags ==")
	c := newTestClient()

	repo, _ := createTestRepo(t, "TestTags", c)

	// Create Tags
	cTagMSG := "A tag message.\n\n:)"
	cTag, resp, err := c.Repositories.CreateTag(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateTagOption{
		TagName: "tag1",
		Message: cTagMSG,
		Target:  "main",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 201, resp.StatusCode)
	assert.EqualValues(t, cTagMSG, cTag.Message)
	assert.EqualValues(t, fmt.Sprintf("%s/%s/TestTags/archive/tag1.zip", getGiteaURL(), repo.Owner.UserName), cTag.ZipballURL)

	tags, _, err := c.Repositories.ListRepoTags(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoTagsOptions{})
	assert.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.EqualValues(t, cTag, tags[0])

	// get tag
	gTag, _, err := c.Repositories.GetTag(t.Context(), repo.Owner.UserName, repo.Name, cTag.Name)
	assert.NoError(t, err)
	assert.EqualValues(t, cTag, gTag)

	aTag, _, err := c.Repositories.GetAnnotatedTag(t.Context(), repo.Owner.UserName, repo.Name, cTag.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, cTag.Name, aTag.Tag)
	assert.EqualValues(t, cTag.ID, aTag.SHA)
	assert.EqualValues(t, fmt.Sprintf("%s/api/v1/repos/%s/TestTags/git/tags/%s", getGiteaURL(), repo.Owner.UserName, cTag.ID), aTag.URL)
	assert.EqualValues(t, cTag.Message+"\n", aTag.Message)
	assert.EqualValues(t, "commit", aTag.Object.Type)

	// DeleteReleaseTag
	resp, err = c.Repositories.DeleteTag(t.Context(), repo.Owner.UserName, repo.Name, "tag1")
	assert.NoError(t, err)
	assert.EqualValues(t, 204, resp.StatusCode)
	tags, _, err = c.Repositories.ListRepoTags(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoTagsOptions{})
	assert.NoError(t, err)
	assert.Len(t, tags, 0)
}
