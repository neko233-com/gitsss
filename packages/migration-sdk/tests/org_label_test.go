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

// TestOrgLabels test label related func
func TestOrgLabels(t *testing.T) {
	log.Println("== TestOrgLabels ==")
	c := newTestClient()
	orgCleanup, orgRepo, err := createTestOrgRepo(t, c, "LabelTestsOrg")
	assert.NoError(t, err)
	defer orgCleanup()

	org := orgRepo.Owner

	createOpts := gitea.CreateOrgLabelOption{
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

	labelOne, _, err := c.Organizations.CreateOrgLabel(t.Context(), org.UserName, createOpts)
	assert.NoError(t, err)
	assert.EqualValues(t, createOpts.Name, labelOne.Name)
	assert.EqualValues(t, createOpts.Color, labelOne.Color)

	labelTwo, _, err := c.Organizations.CreateOrgLabel(t.Context(), org.UserName, gitea.CreateOrgLabelOption{
		Name:        "blue",
		Color:       "#0000FF",
		Description: "CMYB(100%, 100%, 0%, 0%)",
	})
	assert.NoError(t, err)
	_, _, err = c.Organizations.CreateOrgLabel(t.Context(), org.UserName, gitea.CreateOrgLabelOption{
		Name:        "gray",
		Color:       "808080",
		Description: "CMYB(0%, 0%, 0%, 50%)",
	})
	assert.NoError(t, err)
	_, _, err = c.Organizations.CreateOrgLabel(t.Context(), org.UserName, gitea.CreateOrgLabelOption{
		Name:        "green",
		Color:       "#98F76C",
		Description: "CMYB(38%, 0%, 56%, 3%)",
	})
	assert.NoError(t, err)

	labels, resp, err := c.Organizations.ListOrgLabels(t.Context(), org.UserName, gitea.ListOrgLabelsOptions{ListOptions: gitea.ListOptions{PageSize: 3}})
	assert.NoError(t, err)
	assert.Len(t, labels, 3)
	assert.NotNil(t, resp)
	assert.Contains(t, labels, labelTwo)
	assert.NotContains(t, labels, labelOne)

	label, _, err := c.Organizations.GetOrgLabel(t.Context(), org.UserName, labelTwo.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, labelTwo, label)

	label, _, err = c.Organizations.EditOrgLabel(t.Context(), org.UserName, labelTwo.ID, gitea.EditOrgLabelOption{
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
	labels, _, _ = c.Organizations.ListOrgLabels(t.Context(), org.UserName, gitea.ListOrgLabelsOptions{ListOptions: gitea.ListOptions{PageSize: 3}})

	createTestIssue(t, c, org.UserName, orgRepo.Name, "test-issue", "", nil, nil, 0, []int64{label.ID}, false, false)
	issueIndex := int64(1)

	issueLabels, _, err := c.Issues.GetIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 1)
	assert.EqualValues(t, label, issueLabels[0])

	_, _, err = c.Issues.AddIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[0].ID}})
	assert.NoError(t, err)

	issueLabels, _, err = c.Issues.AddIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[1].ID, labels[2].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 3)
	assert.EqualValues(t, labels, issueLabels)

	labels, _, _ = c.Repositories.ListRepoLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, gitea.ListLabelsOptions{})
	assert.Len(t, labels, 7)

	issueLabels, _, err = c.Issues.ReplaceIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.IssueLabelsOption{Labels: []int64{labels[0].ID, labels[1].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 2)

	_, err = c.Issues.DeleteIssueLabel(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, labels[0].ID)
	assert.NoError(t, err)
	issueLabels, _, _ = c.Issues.GetIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.Len(t, issueLabels, 1)

	_, err = c.Issues.ClearIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex)
	assert.NoError(t, err)
	issueLabels, _, _ = c.Issues.GetIssueLabels(t.Context(), orgRepo.Owner.UserName, orgRepo.Name, issueIndex, gitea.ListLabelsOptions{})
	assert.Len(t, issueLabels, 0)

	_, err = c.Organizations.DeleteOrgLabel(t.Context(), org.UserName, labelTwo.ID)
	assert.NoError(t, err)
	labels, _, _ = c.Organizations.ListOrgLabels(t.Context(), org.UserName, gitea.ListOrgLabelsOptions{})
	assert.Len(t, labels, 3)
}
