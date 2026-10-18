// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/db"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTopic(t *testing.T) {
	totalNrOfTopics := 6
	repo1NrOfTopics := 3

	assert.NoError(t, unittest.PrepareTestDatabase())

	topics, err := db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{})
	assert.NoError(t, err)
	assert.Len(t, topics, totalNrOfTopics)

	topics, total, err := db.FindAndCount[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{
		Page: 1, PageSize: 2,
	})
	assert.NoError(t, err)
	assert.Len(t, topics, 2)
	assert.EqualValues(t, 6, total)

	topics, err = db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{
		RepoID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, topics, repo1NrOfTopics)

	assert.NoError(t, repo_model.SaveTopics(t.Context(), 2, "golang"))
	repo2NrOfTopics := 1
	topics, err = db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{})
	assert.NoError(t, err)
	assert.Len(t, topics, totalNrOfTopics)

	topics, err = db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{
		RepoID: 2,
	})
	assert.NoError(t, err)
	assert.Len(t, topics, repo2NrOfTopics)

	assert.NoError(t, repo_model.SaveTopics(t.Context(), 2, "golang", "gitea"))
	repo2NrOfTopics = 2
	totalNrOfTopics++
	topic, err := repo_model.GetTopicByName(t.Context(), "gitea")
	assert.NoError(t, err)
	assert.Equal(t, 1, topic.RepoCount)

	topics, err = db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{})
	assert.NoError(t, err)
	assert.Len(t, topics, totalNrOfTopics)

	topics, err = db.Find[repo_model.Topic](t.Context(), &repo_model.FindTopicOptions{
		RepoID: 2,
	})
	assert.NoError(t, err)
	assert.Len(t, topics, repo2NrOfTopics)
}

func TestCustomTagsSearchAndVisibility(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	const shared = "研发_工具"
	const hidden = "私有构建工具"
	require.NoError(t, repo_model.SaveTopics(t.Context(), 1, shared))
	require.NoError(t, repo_model.SaveTopics(t.Context(), 2, shared, hidden))
	public := repo_model.SearchRepositoryCondition(repo_model.SearchRepoOptions{})
	opts := &repo_model.FindTopicOptions{Keyword: "工具", VisibleRepoCond: public}
	topics, total, err := db.FindAndCount[repo_model.Topic](t.Context(), opts)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, topics, 1)
	assert.Equal(t, shared, topics[0].Name)
	require.NoError(t, repo_model.LoadVisibleTopicCounts(t.Context(), topics, public))
	assert.Equal(t, 1, topics[0].RepoCount)
	repos, _, err := repo_model.SearchRepository(t.Context(), repo_model.SearchRepoOptions{Keyword: shared})
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.EqualValues(t, 1, repos[0].ID)
	valid, invalid := repo_model.SanitizeAndValidateTopics([]string{" 研发_工具 ", "CI_CD", "ci_cd", "bad tag", "研发_工具"})
	assert.Equal(t, []string{shared, "ci_cd"}, valid)
	assert.Equal(t, []string{"bad tag"}, invalid)
}

func TestTopicValidator(t *testing.T) {
	assert.True(t, repo_model.ValidateTopic("游戏客户端"))
	assert.True(t, repo_model.ValidateTopic("ci_cd-构建.工具"))
	assert.True(t, repo_model.ValidateTopic(strings.Repeat("中", 35)))
	assert.False(t, repo_model.ValidateTopic(strings.Repeat("中", 36)))
	assert.False(t, repo_model.ValidateTopic("<script>"))
	assert.False(t, repo_model.ValidateTopic("CI/CD"))
	assert.True(t, repo_model.ValidateTopic("12345"))
	assert.True(t, repo_model.ValidateTopic("2-test"))
	assert.True(t, repo_model.ValidateTopic("foo.bar"))
	assert.True(t, repo_model.ValidateTopic("test-3"))
	assert.True(t, repo_model.ValidateTopic("first"))
	assert.True(t, repo_model.ValidateTopic("second-test-topic"))
	assert.True(t, repo_model.ValidateTopic("third-project-topic-with-max-length"))

	assert.False(t, repo_model.ValidateTopic("$fourth-test,topic"))
	assert.False(t, repo_model.ValidateTopic("-fifth-test-topic"))
	assert.False(t, repo_model.ValidateTopic("sixth-go-project-topic-with-excess-length"))
	assert.False(t, repo_model.ValidateTopic(".foo"))
}
