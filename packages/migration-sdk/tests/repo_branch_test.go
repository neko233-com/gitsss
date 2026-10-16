// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"encoding/base64"
	"log"
	"net/http"
	"testing"
	"time"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestRepoBranches(t *testing.T) {
	log.Println("== TestRepoBranches ==")
	c := newTestClient()
	repoName := "branches"

	repo := prepareBranchTest(t, c, repoName)
	if repo == nil {
		return
	}
	time.Sleep(1 * time.Second)

	// list repo branches
	bl, _, err := c.Repositories.ListRepoBranches(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoBranchesOptions{})
	assert.NoError(t, err)
	assert.Len(t, bl, 3)

	branchNames := make([]string, len(bl))
	branches := make(map[string]gitea.Branch, len(bl))
	for index, branch := range bl {
		branchNames[index] = branch.Name
		branches[branch.Name] = *branch
	}
	assert.ElementsMatch(t, []string{"feature", "main", "update"}, branchNames)

	b, _, err := c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "update")
	assert.NoError(t, err)
	assert.EqualValues(t, branches["update"].Commit.ID, b.Commit.ID)
	assert.EqualValues(t, branches["update"].Commit.Added, b.Commit.Added)

	// try deleting the default branch
	s, _, err := c.Repositories.DeleteRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "main")
	assert.NoError(t, err)
	assert.False(t, s)

	// delete a regular branch
	s, _, err = c.Repositories.DeleteRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "feature")
	assert.NoError(t, err)
	assert.True(t, s)

	bl, _, err = c.Repositories.ListRepoBranches(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListRepoBranchesOptions{})
	assert.NoError(t, err)
	assert.Len(t, bl, 2)

	b, _, err = c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "feature")
	assert.Error(t, err)
	assert.Nil(t, b)

	// create a branch
	bNew, _, err := c.Repositories.CreateBranch(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateBranchOption{BranchName: "NewBranch"})
	assert.NoError(t, err)

	b, _, err = c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, bNew.Name)
	assert.NoError(t, err)
	assert.EqualValues(t, bNew, b)

	// update a branch
	successful, _, err := c.Repositories.RenameRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "NewBranch", gitea.RenameRepoBranchOption{Name: "RenamedBranch"})
	assert.True(t, successful)
	assert.NoError(t, err)

	b, _, err = c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "RenamedBranch")
	assert.NoError(t, err)
	assert.EqualValues(t, "RenamedBranch", b.Name)
}

func TestRepoBranchProtection(t *testing.T) {
	log.Println("== TestRepoBranchProtection ==")
	c := newTestClient()
	repoName := "BranchProtection"

	repo := prepareBranchTest(t, c, repoName)
	if repo == nil {
		return
	}
	assert.NotNil(t, repo)

	// ListBranchProtections
	bpl, _, err := c.Repositories.ListBranchProtections(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListBranchProtectionsOptions{})
	assert.NoError(t, err)
	assert.Len(t, bpl, 0)

	// CreateBranchProtection
	bp, _, err := c.Repositories.CreateBranchProtection(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateBranchProtectionOption{
		BranchName:              "main",
		EnablePush:              true,
		EnablePushWhitelist:     true,
		PushWhitelistUsernames:  []string{"test01"},
		EnableMergeWhitelist:    true,
		MergeWhitelistUsernames: []string{"test01"},
		BlockOnOutdatedBranch:   true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, "main", bp.BranchName)
	assert.EqualValues(t, false, bp.EnableStatusCheck)
	assert.EqualValues(t, true, bp.EnablePush)
	assert.EqualValues(t, true, bp.EnablePushWhitelist)
	assert.EqualValues(t, []string{"test01"}, bp.PushWhitelistUsernames)

	bp, _, err = c.Repositories.CreateBranchProtection(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateBranchProtectionOption{
		BranchName:              "update",
		EnablePush:              false,
		EnableMergeWhitelist:    true,
		MergeWhitelistUsernames: []string{"test01"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, bp)

	bpl, _, err = c.Repositories.ListBranchProtections(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListBranchProtectionsOptions{})
	assert.NoError(t, err)
	assert.Len(t, bpl, 2)

	// GetBranchProtection
	bp, _, err = c.Repositories.GetBranchProtection(t.Context(), repo.Owner.UserName, repo.Name, bpl[0].BranchName)
	assert.NoError(t, err)
	assert.EqualValues(t, bpl[0], bp)

	// EditBranchProtection
	bp, _, err = c.Repositories.EditBranchProtection(t.Context(), repo.Owner.UserName, repo.Name, bpl[0].BranchName, gitea.EditBranchProtectionOption{
		EnablePush:                  gitea.OptionalBool(false),
		EnablePushWhitelist:         gitea.OptionalBool(false),
		PushWhitelistUsernames:      nil,
		RequiredApprovals:           gitea.OptionalInt64(1),
		EnableApprovalsWhitelist:    gitea.OptionalBool(true),
		ApprovalsWhitelistUsernames: []string{"test01"},
	})
	assert.NoError(t, err)
	assert.NotEqual(t, bpl[0], bp)
	assert.EqualValues(t, bpl[0].BranchName, bp.BranchName)
	assert.EqualValues(t, bpl[0].EnableMergeWhitelist, bp.EnableMergeWhitelist)
	assert.EqualValues(t, bpl[0].Created, bp.Created)

	// DeleteBranchProtection
	_, err = c.Repositories.DeleteBranchProtection(t.Context(), repo.Owner.UserName, repo.Name, bpl[1].BranchName)
	assert.NoError(t, err)
	bpl, _, err = c.Repositories.ListBranchProtections(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListBranchProtectionsOptions{})
	assert.NoError(t, err)
	assert.Len(t, bpl, 1)
}

func TestMainNightlyUpdateRepoBranchRef(t *testing.T) {
	log.Println("== TestMainNightlyUpdateRepoBranchRef ==")
	c := newTestClient()
	repoName := "BranchRefMainNightly"

	repo, err := createTestRepo(t, repoName, c)
	if !assert.NoError(t, err) {
		return
	}

	branch, _, err := c.Repositories.CreateBranch(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateBranchOption{BranchName: "feature"})
	assert.NoError(t, err)
	if !assert.NotNil(t, branch) {
		return
	}
	featureInitialCommit := branch.Commit.ID
	assert.NotEmpty(t, featureInitialCommit)

	newFile, _, err := c.Repositories.CreateFile(t.Context(), repo.Owner.UserName, repo.Name, "docs/update.txt", gitea.CreateFileOptions{
		Content: base64.StdEncoding.EncodeToString([]byte("branch update test")),
		FileOptions: gitea.FileOptions{
			BranchName: "main",
			Message:    "add docs/update.txt",
		},
	})
	assert.NoError(t, err)
	if !assert.NotNil(t, newFile) {
		return
	}
	newCommit := newFile.Commit.SHA
	assert.NotEmpty(t, newCommit)

	successful, resp, err := c.Repositories.UpdateRepoBranchRef(t.Context(), repo.Owner.UserName, repo.Name, "feature", gitea.UpdateRepoBranchRefOption{
		NewCommitID: newCommit,
		OldCommitID: featureInitialCommit,
	})
	assert.NoError(t, err)
	assert.True(t, successful)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	}

	updatedBranch, _, err := c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "feature")
	assert.NoError(t, err)
	if assert.NotNil(t, updatedBranch) {
		assert.Equal(t, newCommit, updatedBranch.Commit.ID)
	}

	successful, resp, err = c.Repositories.UpdateRepoBranchRef(t.Context(), repo.Owner.UserName, repo.Name, "feature", gitea.UpdateRepoBranchRefOption{
		NewCommitID: newCommit,
		OldCommitID: featureInitialCommit,
	})
	assert.NoError(t, err)
	assert.False(t, successful)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	}

	successful, resp, err = c.Repositories.UpdateRepoBranchRef(t.Context(), repo.Owner.UserName, repo.Name, "feature", gitea.UpdateRepoBranchRefOption{
		NewCommitID: featureInitialCommit,
		OldCommitID: newCommit,
	})
	assert.NoError(t, err)
	assert.False(t, successful)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	}

	successful, resp, err = c.Repositories.UpdateRepoBranchRef(t.Context(), repo.Owner.UserName, repo.Name, "feature", gitea.UpdateRepoBranchRefOption{
		NewCommitID: featureInitialCommit,
		OldCommitID: newCommit,
		Force:       true,
	})
	assert.NoError(t, err)
	assert.True(t, successful)
	if assert.NotNil(t, resp) {
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	}

	updatedBranch, _, err = c.Repositories.GetRepoBranch(t.Context(), repo.Owner.UserName, repo.Name, "feature")
	assert.NoError(t, err)
	if assert.NotNil(t, updatedBranch) {
		assert.Equal(t, featureInitialCommit, updatedBranch.Commit.ID)
	}
}

func prepareBranchTest(t *testing.T, c *gitea.Client, repoName string) *gitea.Repository {
	origRepo, err := createTestRepo(t, repoName, c)
	if !assert.NoError(t, err) {
		return nil
	}

	mainLicense, _, err := c.Repositories.GetContents(t.Context(), origRepo.Owner.UserName, origRepo.Name, "main", "README.md")
	if !assert.NoError(t, err) || !assert.NotNil(t, mainLicense) {
		return nil
	}

	updatedFile, _, err := c.Repositories.UpdateFile(t.Context(), origRepo.Owner.UserName, origRepo.Name, "README.md", gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:       "update it",
			BranchName:    "main",
			NewBranchName: "update",
		},
		SHA:     mainLicense.SHA,
		Content: "Tk9USElORyBJUyBIRVJFIEFOWU1PUkUKSUYgWU9VIExJS0UgVE8gRklORCBTT01FVEhJTkcKV0FJVCBGT1IgVEhFIEZVVFVSRQo=",
	})
	if !assert.NoError(t, err) || !assert.NotNil(t, updatedFile) {
		return nil
	}

	newFile, _, err := c.Repositories.CreateFile(t.Context(), origRepo.Owner.UserName, origRepo.Name, "WOW-file", gitea.CreateFileOptions{
		Content: "QSBuZXcgRmlsZQo=",
		FileOptions: gitea.FileOptions{
			Message:       "creat a new file",
			BranchName:    "main",
			NewBranchName: "feature",
		},
	})
	if !assert.NoError(t, err) || !assert.NotNil(t, newFile) {
		return nil
	}

	return origRepo
}
