// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

// TestIssueComment creat a issue and test comment creation/edit/deletion on it
func TestIssueTimeline(t *testing.T) {
	log.Println("== TestIssueTimeline ==")

	c := newTestClient()

	user, _, err := c.Users.GetMyUserInfo(t.Context())

	assert.NoError(t, err)
	repo, err := createTestRepo(t, "TestIssueCommentRepo", c)
	assert.NoError(t, err)
	issue1, _, err := c.Issues.CreateIssue(t.Context(), user.UserName, repo.Name, gitea.CreateIssueOption{Title: "issue1", Body: "body", Closed: false})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, issue1.Index)
	issue2, _, err := c.Issues.CreateIssue(t.Context(), user.UserName, repo.Name, gitea.CreateIssueOption{Title: "issue1", Body: "body", Closed: false})
	assert.EqualValues(t, 2, issue2.Index)
	assert.NoError(t, err)
	tUser2 := createTestUser(t, "Commenter2", c)
	tUser3 := createTestUser(t, "Commenter3", c)

	createOne := func(u *gitea.User, issue int64, text string) {
		c.SetSudo(u.UserName)
		comment, _, e := c.Issues.CreateIssueComment(t.Context(), user.UserName, repo.Name, issue, gitea.CreateIssueCommentOption{Body: text})
		c.SetSudo("")
		assert.NoError(t, e)
		assert.NotEmpty(t, comment)
		assert.EqualValues(t, text, comment.Body)
		assert.EqualValues(t, u.ID, comment.Poster.ID)
	}

	// CreateIssue
	createOne(user, 1, "what a nice issue")
	createOne(tUser2, 1, "dont think so")
	createOne(tUser3, 1, "weow weow")
	createOne(user, 1, "spam isn't it?")
	createOne(tUser3, 2, "hehe first commit")
	createOne(tUser2, 2, "second")
	createOne(user, 2, "3")

	_, err = c.Admin.DeleteUser(t.Context(), tUser3.UserName)
	assert.NoError(t, err)

	// ListIssueComments
	comments, _, err := c.Issues.ListIssueTimeline(t.Context(), user.UserName, repo.Name, 2, gitea.ListIssueCommentOptions{})
	assert.NoError(t, err)
	assert.Len(t, comments, 3)
}
