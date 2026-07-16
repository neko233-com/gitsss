// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package stats

import (
	"testing"
	"time"

	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/queue"
	"github.com/neko233-com/gitsss/modules/setting"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/actions"
	_ "github.com/neko233-com/gitsss/models/activities"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRepoStatsIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.CfgProvider, _ = setting.NewConfigProviderFromData("")

	setting.LoadQueueSettings()

	err := Init()
	assert.NoError(t, err)

	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)

	err = UpdateRepoIndexer(repo)
	assert.NoError(t, err)

	assert.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))

	status, err := repo_model.GetIndexerStatus(t.Context(), repo, repo_model.RepoIndexerTypeStats)
	assert.NoError(t, err)
	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", status.CommitSha)
	langs, err := repo_model.GetTopLanguageStats(t.Context(), repo, 5)
	assert.NoError(t, err)
	assert.Empty(t, langs)
}
