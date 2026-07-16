// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

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

	createOpts := CreateOrgLabelOption{
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

	labelOne, _, err := c.CreateOrgLabel(org.UserName, createOpts)
	assert.NoError(t, err)
	assert.EqualValues(t, createOpts.Name, labelOne.Name)
	assert.EqualValues(t, createOpts.Color, labelOne.Color)

	labelTwo, _, err := c.CreateOrgLabel(org.UserName, CreateOrgLabelOption{
		Name:        "blue",
		Color:       "#0000FF",
		Description: "CMYB(100%, 100%, 0%, 0%)",
	})
	assert.NoError(t, err)
	_, _, err = c.CreateOrgLabel(org.UserName, CreateOrgLabelOption{
		Name:        "gray",
		Color:       "808080",
		Description: "CMYB(0%, 0%, 0%, 50%)",
	})
	assert.NoError(t, err)
	_, _, err = c.CreateOrgLabel(org.UserName, CreateOrgLabelOption{
		Name:        "green",
		Color:       "#98F76C",
		Description: "CMYB(38%, 0%, 56%, 3%)",
	})
	assert.NoError(t, err)

	labels, resp, err := c.ListOrgLabels(org.UserName, ListOrgLabelsOptions{ListOptions: ListOptions{PageSize: 3}})
	assert.NoError(t, err)
	assert.Len(t, labels, 3)
	assert.NotNil(t, resp)
	assert.Contains(t, labels, labelTwo)
	assert.NotContains(t, labels, labelOne)

	label, _, err := c.GetOrgLabel(org.UserName, labelTwo.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, labelTwo, label)

	label, _, err = c.EditOrgLabel(org.UserName, labelTwo.ID, EditOrgLabelOption{
		Color:       OptionalString("#0e0175"),
		Description: OptionalString("blueish"),
	})
	assert.NoError(t, err)
	assert.EqualValues(t, &Label{
		ID:          labelTwo.ID,
		Name:        labelTwo.Name,
		Color:       "0e0175",
		Description: "blueish",
		URL:         labelTwo.URL,
	}, label)
	labels, _, _ = c.ListOrgLabels(org.UserName, ListOrgLabelsOptions{ListOptions: ListOptions{PageSize: 3}})

	createTestIssue(t, c, org.UserName, orgRepo.Name, "test-issue", "", nil, nil, 0, []int64{label.ID}, false, false)
	issueIndex := int64(1)

	issueLabels, _, err := c.GetIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, ListLabelsOptions{})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 1)
	assert.EqualValues(t, label, issueLabels[0])

	_, _, err = c.AddIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, IssueLabelsOption{Labels: []int64{labels[0].ID}})
	assert.NoError(t, err)

	issueLabels, _, err = c.AddIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, IssueLabelsOption{Labels: []int64{labels[1].ID, labels[2].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 3)
	assert.EqualValues(t, labels, issueLabels)

	labels, _, _ = c.ListRepoLabels(orgRepo.Owner.UserName, orgRepo.Name, ListLabelsOptions{})
	assert.Len(t, labels, 7)

	issueLabels, _, err = c.ReplaceIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, IssueLabelsOption{Labels: []int64{labels[0].ID, labels[1].ID}})
	assert.NoError(t, err)
	assert.Len(t, issueLabels, 2)

	_, err = c.DeleteIssueLabel(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, labels[0].ID)
	assert.NoError(t, err)
	issueLabels, _, _ = c.GetIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, ListLabelsOptions{})
	assert.Len(t, issueLabels, 1)

	_, err = c.ClearIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex)
	assert.NoError(t, err)
	issueLabels, _, _ = c.GetIssueLabels(orgRepo.Owner.UserName, orgRepo.Name, issueIndex, ListLabelsOptions{})
	assert.Len(t, issueLabels, 0)

	_, err = c.DeleteOrgLabel(org.UserName, labelTwo.ID)
	assert.NoError(t, err)
	labels, _, _ = c.ListOrgLabels(org.UserName, ListOrgLabelsOptions{})
	assert.Len(t, labels, 3)
}
