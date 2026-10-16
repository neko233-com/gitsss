// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoWatch(t *testing.T) {
	log.Printf("== TestRepoWatch ==")
	c := newTestClient()
	rawVersion, _, err := c.Meta.ServerVersion(t.Context())
	assert.NoError(t, err)
	assert.True(t, true, rawVersion != "")

	repo1, _ := createTestRepo(t, "TestRepoWatch_1", c)
	repo2, _ := createTestRepo(t, "TestRepoWatch_2", c)
	assert.NotEqual(t, repo1, repo2)

	// GetWatchedRepos
	wl, _, err := c.Repositories.GetWatchedRepos(t.Context(), "test01")
	assert.NoError(t, err)
	assert.NotNil(t, wl)
	maxcount := len(wl)

	// GetMyWatchedRepos
	wl, _, err = c.Repositories.GetMyWatchedRepos(t.Context())
	assert.NoError(t, err)
	assert.Len(t, wl, maxcount)

	// CheckRepoWatch
	isWatching, _, err := c.Repositories.CheckRepoWatch(t.Context(), repo1.Owner.UserName, repo1.Name)
	assert.NoError(t, err)
	assert.True(t, isWatching)

	// UnWatchRepo
	_, err = c.Repositories.UnWatchRepo(t.Context(), repo1.Owner.UserName, repo1.Name)
	assert.NoError(t, err)
	isWatching, _, _ = c.Repositories.CheckRepoWatch(t.Context(), repo1.Owner.UserName, repo1.Name)
	assert.False(t, isWatching)

	// WatchRepo
	_, err = c.Repositories.WatchRepo(t.Context(), repo1.Owner.UserName, repo1.Name)
	assert.NoError(t, err)
	isWatching, _, _ = c.Repositories.CheckRepoWatch(t.Context(), repo1.Owner.UserName, repo1.Name)
	assert.True(t, isWatching)
}
