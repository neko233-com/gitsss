// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"
	"time"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

// TestIssue is main func witch call all Tests for Issue API
// (to make sure they are on correct order)
func TestIssue(t *testing.T) {
	c := newTestClient()

	createIssue(t, c)
	// Little sleep in order to give some time for gitea to properly store all information on database. Without this sleep, CI is a bit unstable
	time.Sleep(1 * time.Second)
	editIssues(t, c)
	listIssues(t, c)
	deleteIssue(t, c)
}

func createIssue(t *testing.T, c *gitea.Client) {
	log.Println("== TestCreateIssues ==")

	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)
	repo, _ := createTestRepo(t, "IssueTestsRepo", c)

	nowTime := time.Now()
	mile, _, _ := c.Repositories.CreateMilestone(t.Context(), user.UserName, repo.Name, gitea.CreateMilestoneOption{Title: "mile1"})
	label1, _, _ := c.Repositories.CreateLabel(t.Context(), user.UserName, repo.Name, gitea.CreateLabelOption{Name: "Label1", Description: "a", Color: "#ee0701"})
	label2, _, _ := c.Repositories.CreateLabel(t.Context(), user.UserName, repo.Name, gitea.CreateLabelOption{Name: "Label2", Description: "b", Color: "#128a0c"})

	createTestIssue(t, c, user.UserName, repo.Name, "First Issue", "", nil, nil, 0, nil, false, false)
	createTestIssue(t, c, user.UserName, repo.Name, "Issue 2", "closed isn't it?", nil, nil, 0, nil, true, false)
	createTestIssue(t, c, user.UserName, repo.Name, "Issue 3", "", nil, nil, 0, nil, true, false)
	createTestIssue(t, c, user.UserName, repo.Name, "Feature: spam protect 4", "explain explain explain", []string{user.UserName}, &nowTime, 0, nil, true, false)
	createTestIssue(t, c, user.UserName, repo.Name, "W 123", "", nil, &nowTime, mile.ID, nil, false, false)
	createTestIssue(t, c, user.UserName, repo.Name, "First Issue", "", nil, nil, 0, nil, false, false)
	createTestIssue(t, c, user.UserName, repo.Name, "Do it soon!", "is important!", []string{user.UserName}, &nowTime, mile.ID, []int64{label1.ID, label2.ID}, false, false)
	createTestIssue(t, c, user.UserName, repo.Name, "Job Done", "you never know", nil, nil, mile.ID, []int64{label2.ID}, true, false)
	createTestIssue(t, c, user.UserName, repo.Name, "", "you never know", nil, nil, mile.ID, nil, true, true)
}

func deleteIssue(t *testing.T, c *gitea.Client) {
	log.Println("== TestDeleteIssues ==")

	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)
	repo, _ := createTestRepo(t, "IssueTestsRepo", c)

	issue := createTestIssue(t, c, user.UserName, repo.Name, "Deleteable Issue", "", nil, nil, 0, nil, false, false)
	_, err = c.Issues.DeleteIssue(t.Context(), user.UserName, repo.Name, issue.Index)
	assert.NoError(t, err)
}

func editIssues(t *testing.T, c *gitea.Client) {
	log.Println("== TestEditIssues ==")
	il, _, err := c.Issues.ListIssues(t.Context(), gitea.ListIssueOption{KeyWord: "Do it soon!"})
	assert.NoError(t, err)
	assert.Len(t, il, 1)
	issue, _, err := c.Issues.GetIssue(t.Context(), il[0].Poster.UserName, il[0].Repository.Name, il[0].Index)
	assert.NoError(t, err)

	state := gitea.StateClosed
	issueNew, _, err := c.Issues.EditIssue(t.Context(), issue.Poster.UserName, issue.Repository.Name, issue.Index, gitea.EditIssueOption{
		Title: "Edited",
		Body:  gitea.OptionalString("123 test and go"),
		State: &state,
		Ref:   gitea.OptionalString("main"),
	})
	assert.NoError(t, err)
	assert.EqualValues(t, issue.ID, issueNew.ID)
	assert.EqualValues(t, "123 test and go", issueNew.Body)
	assert.EqualValues(t, "Edited", issueNew.Title)
	assert.EqualValues(t, "main", issueNew.Ref)
}

func listIssues(t *testing.T, c *gitea.Client) {
	log.Println("== TestListIssues ==")

	issues, _, err := c.Issues.ListRepoIssues(t.Context(), "test01", "IssueTestsRepo", gitea.ListIssueOption{
		Labels:  []string{"Label1", "Label2"},
		KeyWord: "",
		State:   "all",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 1)

	issues, _, err = c.Issues.ListIssues(t.Context(), gitea.ListIssueOption{
		Labels:  []string{"Label2"},
		KeyWord: "Done",
		State:   "all",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 1)

	issues, _, err = c.Issues.ListRepoIssues(t.Context(), "test01", "IssueTestsRepo", gitea.ListIssueOption{
		Milestones: []string{"mile1"},
		State:      "all",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 3)
	for i := range issues {
		if assert.NotNil(t, issues[i].Milestone) {
			assert.EqualValues(t, "mile1", issues[i].Milestone.Title)
		}
	}

	issues, _, err = c.Issues.ListRepoIssues(t.Context(), "test01", "IssueTestsRepo", gitea.ListIssueOption{})
	assert.NoError(t, err)
	assert.Len(t, issues, 3)
}

func createTestIssue(t *testing.T, c *gitea.Client, userName, repoName, title, body string, assignees []string, deadline *time.Time, milestone int64, labels []int64, closed, shouldFail bool) *gitea.Issue {
	issue, _, e := c.Issues.CreateIssue(t.Context(), userName, repoName, gitea.CreateIssueOption{
		Title:     title,
		Body:      body,
		Assignees: assignees,
		Deadline:  deadline,
		Milestone: milestone,
		Labels:    labels,
		Closed:    closed,
	})
	if shouldFail {
		assert.Error(t, e)
		return nil
	}
	assert.NoError(t, e)
	assert.NotEmpty(t, issue)
	assert.EqualValues(t, title, issue.Title)
	assert.EqualValues(t, body, issue.Body)
	assert.EqualValues(t, len(assignees), len(issue.Assignees))
	for i, a := range issue.Assignees {
		assert.EqualValues(t, assignees[i], a.UserName)
	}
	if milestone > 0 {
		assert.EqualValues(t, milestone, issue.Milestone.ID)
	}
	assert.EqualValues(t, len(labels), len(issue.Labels))
	if closed {
		assert.False(t, issue.Closed.IsZero())
	} else {
		assert.Empty(t, issue.Closed)
	}
	return issue
}
