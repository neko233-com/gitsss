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

func TestCommitStatus(t *testing.T) {
	log.Println("== TestCommitStatus ==")
	c := newTestClient()
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	repoName := "CommitStatuses"
	origRepo, err := createTestRepo(t, repoName, c)
	if !assert.NoError(t, err) {
		return
	}

	commits, _, _ := c.Repositories.ListRepoCommits(t.Context(), user.UserName, repoName, gitea.ListCommitOptions{
		ListOptions: gitea.ListOptions{},
		SHA:         origRepo.DefaultBranch,
	})
	if !assert.Len(t, commits, 1) {
		return
	}
	sha := commits[0].SHA

	combiStats, resp, err := c.Repositories.GetCombinedStatus(t.Context(), user.UserName, repoName, sha)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, combiStats)
	assert.EqualValues(t, 0, combiStats.TotalCount)

	statuses, resp, err := c.Repositories.ListStatuses(t.Context(), user.UserName, repoName, sha, gitea.ListStatusesOption{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, statuses)
	assert.Len(t, statuses, 0)

	createStatus(t, c, user.UserName, repoName, sha, "http://dummy.test", "start testing", "ultraCI", gitea.StatusPending)
	createStatus(t, c, user.UserName, repoName, sha, "https://more.secure", "just a warning", "warn/bot", gitea.StatusWarning)
	createStatus(t, c, user.UserName, repoName, sha, "http://dummy.test", "test failed", "ultraCI", gitea.StatusFailure)
	createStatus(t, c, user.UserName, repoName, sha, "http://dummy.test", "start testing", "ultraCI", gitea.StatusPending)
	createStatus(t, c, user.UserName, repoName, sha, "http://dummy.test", "test passed", "ultraCI", gitea.StatusSuccess)

	statuses, resp, err = c.Repositories.ListStatuses(t.Context(), user.UserName, repoName, sha, gitea.ListStatusesOption{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, statuses)
	assert.Len(t, statuses, 5)

	combiStats, resp, err = c.Repositories.GetCombinedStatus(t.Context(), user.UserName, repoName, sha)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, combiStats)
	assert.EqualValues(t, 2, combiStats.TotalCount)
	assert.EqualValues(t, gitea.StatusState("failure"), combiStats.State)
	assert.Len(t, combiStats.Statuses, 2)
}

func createStatus(t *testing.T, c *gitea.Client, userName, repoName, sha, url, desc, statusContext string, state gitea.StatusState) {
	stats, resp, err := c.Repositories.CreateStatus(t.Context(), userName, repoName, sha, gitea.CreateStatusOption{
		State:       state,
		TargetURL:   url,
		Description: desc,
		Context:     statusContext,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, stats)
	assert.EqualValues(t, state, stats.State)
}
