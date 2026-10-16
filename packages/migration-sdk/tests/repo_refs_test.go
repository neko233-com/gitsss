// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"net/http"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoRefs(t *testing.T) {
	log.Println("== TestRepoRefs ==")
	c := newTestClient()

	repo, err := createTestRepo(t, "TestRepoRefs", c)
	require.NoError(t, err)

	branch, _, err := c.Repositories.CreateBranch(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateBranchOption{BranchName: "feature"})
	require.NoError(t, err)
	require.NotNil(t, branch)

	tag, _, err := c.Repositories.CreateTag(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateTagOption{
		TagName: "v1.0.0",
		Target:  "main",
	})
	require.NoError(t, err)
	require.NotNil(t, tag)

	allRefs, _, err := c.Git.ListAllGitRefs(t.Context(), repo.Owner.UserName, repo.Name)
	require.NoError(t, err)
	assertRefNamesContain(t, allRefs, "refs/heads/main", "refs/heads/feature", "refs/tags/v1.0.0")

	branchRefs, _, err := c.Git.GetRepoRefs(t.Context(), repo.Owner.UserName, repo.Name, "heads")
	require.NoError(t, err)
	assertRefNamesContain(t, branchRefs, "refs/heads/main", "refs/heads/feature")
	assertRefNamesDoNotContain(t, branchRefs, "refs/tags/v1.0.0")

	featureRef, _, err := c.Git.GetRepoRef(t.Context(), repo.Owner.UserName, repo.Name, "heads/feature")
	require.NoError(t, err)
	require.NotNil(t, featureRef)
	require.NotNil(t, featureRef.Object)
	assert.Equal(t, "refs/heads/feature", featureRef.Ref)
	assert.Equal(t, branch.Commit.ID, featureRef.Object.SHA)

	tagRef, _, err := c.Git.GetRepoRef(t.Context(), repo.Owner.UserName, repo.Name, "refs/tags/v1.0.0")
	require.NoError(t, err)
	require.NotNil(t, tagRef)
	require.NotNil(t, tagRef.Object)
	assert.Equal(t, "refs/tags/v1.0.0", tagRef.Ref)
	assert.NotEmpty(t, tagRef.Object.SHA)

	missingExactRef, _, err := c.Git.GetRepoRef(t.Context(), repo.Owner.UserName, repo.Name, "heads")
	assert.Nil(t, missingExactRef)
	assert.EqualError(t, err, "no exact match found for this ref")

	notFoundRef, resp, err := c.Git.GetRepoRef(t.Context(), repo.Owner.UserName, repo.Name, "heads/does-not-exist")
	assert.Nil(t, notFoundRef)
	assert.Error(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
}

func assertRefNamesContain(t *testing.T, refs []*gitea.Reference, expectedRefs ...string) {
	t.Helper()

	refNames := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		refNames = append(refNames, ref.Ref)
	}

	for _, expectedRef := range expectedRefs {
		assert.Contains(t, refNames, expectedRef)
	}
}

func assertRefNamesDoNotContain(t *testing.T, refs []*gitea.Reference, unexpectedRefs ...string) {
	t.Helper()

	refNames := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		refNames = append(refNames, ref.Ref)
	}

	for _, unexpectedRef := range unexpectedRefs {
		assert.NotContains(t, refNames, unexpectedRef)
	}
}
