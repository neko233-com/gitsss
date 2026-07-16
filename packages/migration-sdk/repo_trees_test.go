// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoGetTrees(t *testing.T) {
	log.Printf("== TestRepoGetTrees ==")
	c := newTestClient()

	repo, err := createTestRepo(t, "TestRepoGetTrees", c)
	assert.NoError(t, err)

	res, _, err := c.GetTrees(repo.Owner.UserName, repo.Name, ListTreeOptions{
		ListOptions: ListOptions{
			Page:     1,
			PageSize: 10,
		},
		Ref:       "main",
		Recursive: true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, res.TotalCount)
	assert.Len(t, res.Entries, 3)
	assert.EqualValues(t, ".gitignore", res.Entries[0].Path)
	assert.EqualValues(t, "LICENSE", res.Entries[1].Path)
	assert.EqualValues(t, "README.md", res.Entries[2].Path)
}
