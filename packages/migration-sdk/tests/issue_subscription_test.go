// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIssue is main func witch call all Tests for Issue API
// (to make sure they are on correct order)
func TestIssueSubscription(t *testing.T) {
	log.Println("== TestIssueSubscription ==")

	c := newTestClient()
	repo, _ := createTestRepo(t, "IssueWatch", c)
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)
	createTestIssue(t, c, user.UserName, repo.Name, "First Issue", "", nil, nil, 0, nil, false, false)

	wi, _, err := c.Issues.CheckIssueSubscription(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	assert.True(t, wi.Subscribed)

	_, err = c.Repositories.UnWatchRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	wi, _, err = c.Issues.CheckIssueSubscription(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	assert.True(t, wi.Subscribed)

	_, err = c.Issues.IssueSubscribe(t.Context(), repo.Owner.UserName, repo.Name, 1)
	if assert.Error(t, err) {
		assert.EqualValues(t, "already subscribed", err.Error())
	}
	wi, _, err = c.Issues.CheckIssueSubscription(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	assert.True(t, wi.Subscribed)

	_, err = c.Issues.IssueUnSubscribe(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	wi, _, err = c.Issues.CheckIssueSubscription(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	assert.False(t, wi.Subscribed)

	_, err = c.Repositories.WatchRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	wi, _, err = c.Issues.CheckIssueSubscription(t.Context(), repo.Owner.UserName, repo.Name, 1)
	assert.NoError(t, err)
	assert.False(t, wi.Subscribed)
}
