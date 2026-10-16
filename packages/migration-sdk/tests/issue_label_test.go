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

// TestIssueLabels test label related func
func TestIssueLabels(t *testing.T) {
	log.Println("== TestIssueLabels ==")
	c := newTestClient()
	repo, err := createTestRepo(t, "LabelTestsRepo", c)
	assert.NoError(t, err)

	createOpts := gitea.CreateLabelOption{
		Name:        " ",
		Description: "",
		Color:       "",
	}
	err = createOpts.Validate()
	assert.Error(t, err)
	assert.EqualValues(t, "invalid color format", err.Error())
	createOpts.Color = "12345f"
	err = createOpts.Validate()
	assert.Error(t, err)
	assert.EqualValues(t, "empty name not allowed", err.Error())
	createOpts.Name = "label one"

	labelOne, _, err := c.Repositories.CreateLabel(t.Context(), repo.Owner.UserName, repo.Name, createOpts)
	assert.NoError(t, err)
	assert.EqualValues(t, createOpts.Name, labelOne.Name)
	assert.EqualValues(t, createOpts.Color, labelOne.Color)

	labelTwo, _, err := c.Repositories.CreateLabel(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateLabelOption{
		Name:        "blue",
		Color:       "#0000FF",
		Description: "CMYB(100%, 100%, 0%, 0%)",
	})
	assert.NoError(t, err)
	_, _, err = c.Repositories.CreateLabel(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateLabelOption{
		Name:        "gray",
		Color:       "808080",
		Description: "CMYB(0%, 0%, 0%, 50%)",
	})
	assert.NoError(t, err)
	_, _, err = c.Repositories.CreateLabel(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateLabelOption{
		Name:        "green",
		Color:       "#98F76C",
		Description: "CMYB(38%, 0%, 56%, 3%)",
	})
	assert.NoError(t, err)

	labels, resp, err := c.Repositories.ListRepoLabels(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListLabelsOptions{ListOptions: gitea.ListOptions{PageSize: 3}})
	assert.NoError(t, err)
	assert.Len(t, labels, 3)
	assert.NotNil(t, resp)
	assert.Contains(t, labels, labelTwo)
	assert.NotContains(t, labels, labelOne)

	label, _, err := c.Repositories.GetRepoLabel(t.Context(), repo.Owner.UserName, repo.Name, labelTwo.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, labelTwo, label)

	label, _, err = c.Repositories.EditLabel(t.Context(), repo.Owner.UserName, repo.Name, labelTwo.ID, gitea.EditLabelOption{
		Color:       gitea.OptionalString("#0e0175"),
		Description: gitea.OptionalString("blueish"),
	})
	assert.NoError(t, err)
	assert.EqualValues(t, &gitea.Label{
		ID:          labelTwo.ID,
		Name:        labelTwo.Name,
		Color:       "0e0175",
		Description: "blueish",
		URL:         labelTwo.URL,
	}, label)
	labels, _, _ = c.Repositories.ListRepoLabels(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListLabelsOptions{ListOptions: gitea.ListOptions{PageSize: 3}})

	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	createTestIssue(t, c, user.UserName, repo.Name, "test-issue", "", nil, nil, 0, []int64{label.ID}, false, false)
	issueIndex := int64(1)

	issueLabels, _, err := c.Issues.GetIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 1)
	assert.EqualValues(t, label, issueLabels[0])

	_, _, err = c.Issues.AddIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[0].ID}})
	assert.NoError(t, err)

	issueLabels, _, err = c.Issues.AddIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[1].ID, labels[2].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 3)
	assert.EqualValues(t, labels, issueLabels)

	labels, _, _ = c.Repositories.ListRepoLabels(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListLabelsOptions{})
	assert.Len(t, labels, 11)

	issueLabels, _, err = c.Issues.ReplaceIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[0].ID, labels[1].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 2)

	_, err = c.Issues.DeleteIssueLabel(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, labels[0].ID)
	assert.NoError(t, err)
	issueLabels, _, _ = c.Issues.GetIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.Len(t, issueLabels, 1)

	_, err = c.Issues.ClearIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex)
	assert.NoError(t, err)
	issueLabels, _, _ = c.Issues.GetIssueLabels(t.Context(), repo.Owner.UserName, repo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.Len(t, issueLabels, 0)

	_, err = c.Repositories.DeleteLabel(t.Context(), repo.Owner.UserName, repo.Name, labelTwo.ID)
	assert.NoError(t, err)
	labels, _, _ = c.Repositories.ListRepoLabels(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListLabelsOptions{})
	assert.Len(t, labels, 10)
}
