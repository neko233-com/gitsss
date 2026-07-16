// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIssueComment creat a issue and test comment creation/edit/deletion on it
func TestIssueTimeline(t *testing.T) {
	log.Println("== TestIssueTimeline ==")

	c := newTestClient()

	user, _, err := c.GetMyUserInfo()

	assert.NoError(t, err)
	repo, err := createTestRepo(t, "TestIssueCommentRepo", c)
	assert.NoError(t, err)
	issue1, _, err := c.CreateIssue(user.UserName, repo.Name, CreateIssueOption{Title: "issue1", Body: "body", Closed: false})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, issue1.Index)
	issue2, _, err := c.CreateIssue(user.UserName, repo.Name, CreateIssueOption{Title: "issue1", Body: "body", Closed: false})
	assert.EqualValues(t, 2, issue2.Index)
	assert.NoError(t, err)
	tUser2 := createTestUser(t, "Commenter2", c)
	tUser3 := createTestUser(t, "Commenter3", c)

	createOne := func(u *User, issue int64, text string) {
		c.sudo = u.UserName
		comment, _, e := c.CreateIssueComment(user.UserName, repo.Name, issue, CreateIssueCommentOption{Body: text})
		c.sudo = ""
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

	_, err = c.AdminDeleteUser(tUser3.UserName)
	assert.NoError(t, err)

	// ListIssueComments
	comments, _, err := c.ListIssueTimeline(user.UserName, repo.Name, 2, ListIssueCommentOptions{})
	assert.NoError(t, err)
	assert.Len(t, comments, 3)
}
