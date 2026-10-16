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

// TestIssueComment creat a issue and test comment creation/edit/deletion on it
func TestIssueComment(t *testing.T) {
	log.Println("== TestIssueComment ==")

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

	// ListRepoIssueComments
	comments, _, err := c.Issues.ListRepoIssueComments(t.Context(), user.UserName, repo.Name, gitea.ListIssueCommentOptions{})
	assert.NoError(t, err)
	assert.Len(t, comments, 7)

	// ListIssueComments
	comments, _, err = c.Issues.ListIssueComments(t.Context(), user.UserName, repo.Name, 2, gitea.ListIssueCommentOptions{})
	assert.NoError(t, err)
	assert.Len(t, comments, 3)

	// GetIssueComment
	comment, _, err := c.Issues.GetIssueComment(t.Context(), user.UserName, repo.Name, comments[1].ID)
	assert.NoError(t, err)
	assert.EqualValues(t, comment.Poster.ID, comments[1].Poster.ID)
	assert.EqualValues(t, comment.Body, comments[1].Body)
	assert.EqualValues(t, comment.Updated.Unix(), comments[1].Updated.Unix())

	// EditIssueComment
	comment, _, err = c.Issues.EditIssueComment(t.Context(), user.UserName, repo.Name, comments[1].ID, gitea.EditIssueCommentOption{
		Body: "changed my mind",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, "changed my mind", comment.Body)

	// DeleteIssueComment
	_, err = c.Issues.DeleteIssueComment(t.Context(), user.UserName, repo.Name, comments[1].ID)
	assert.NoError(t, err)
	_, _, err = c.Issues.GetIssueComment(t.Context(), user.UserName, repo.Name, comments[1].ID)
	assert.Error(t, err)
}
