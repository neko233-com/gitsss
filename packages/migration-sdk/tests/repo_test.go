// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"bytes"
	"io"
	"log"
	"testing"
	"time"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestCreateRepo(t *testing.T) {
	log.Println("== TestCreateRepo ==")
	c := newTestClient()
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	repoName := "test1"
	_, _, err = c.Repositories.GetRepo(t.Context(), user.UserName, repoName)
	if err != nil {
		repo, _, err := c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
			Name: repoName,
		})
		assert.NoError(t, err)
		assert.NotNil(t, repo)
	}

	_, err = c.Repositories.DeleteRepo(t.Context(), user.UserName, repoName)
	assert.NoError(t, err)
}

func TestCreateRepoSha256(t *testing.T) {
	log.Println("== TestCreateRepoSha256 ==")
	c := newTestClient()
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	repoName := "test1"
	_, _, err = c.Repositories.GetRepo(t.Context(), user.UserName, repoName)
	if err != nil {
		repo, _, err := c.Repositories.CreateRepo(t.Context(), gitea.CreateRepoOption{
			Name:             repoName,
			ObjectFormatName: "sha256",
		})
		assert.NoError(t, err)
		assert.NotNil(t, repo)
		assert.EqualValues(t, "sha256", repo.ObjectFormatName)
	}

	_, err = c.Repositories.DeleteRepo(t.Context(), user.UserName, repoName)
	assert.NoError(t, err)
}

func TestRepoMigrateAndLanguages(t *testing.T) {
	log.Println("== TestMigrateRepo ==")
	c := newTestClient()
	user, _, uErr := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, uErr)
	_, _, err := c.Repositories.GetRepo(t.Context(), user.UserName, "sdk-mirror")
	if err == nil {
		_, _ = c.Repositories.DeleteRepo(t.Context(), user.UserName, "sdk-mirror")
	}

	repoM, _, err := c.Repositories.MigrateRepo(t.Context(), gitea.MigrateRepoOption{
		CloneAddr:   "https://gitea.com/gitea/go-sdk.git",
		RepoName:    "sdk-mirror",
		RepoOwner:   user.UserName,
		Mirror:      true,
		Private:     false,
		Description: "mirror sdk",
	})
	assert.NoError(t, err)

	repoG, _, err := c.Repositories.GetRepo(t.Context(), repoM.Owner.UserName, repoM.Name)
	assert.NoError(t, err)
	assert.EqualValues(t, repoM.ID, repoG.ID)
	assert.EqualValues(t, "main", repoG.DefaultBranch)
	assert.True(t, repoG.Mirror)
	assert.False(t, repoG.Empty)
	assert.EqualValues(t, 1, repoG.Watchers)
	var zeroTime time.Time
	assert.NotEqual(t, zeroTime, repoG.MirrorUpdated)

	log.Println("== TestRepoLanguages ==")
	time.Sleep(time.Second * 2)
	lang, _, err := c.Repositories.GetRepoLanguages(t.Context(), repoM.Owner.UserName, repoM.Name)
	assert.NoError(t, err)
	assert.Len(t, lang, 2)
	assert.True(t, 217441 < lang["Go"])
	assert.True(t, 3614 < lang["Makefile"] && 6000 > lang["Makefile"])
}

func TestSearchRepo(t *testing.T) {
	log.Println("== TestSearchRepo ==")
	c := newTestClient()

	repo, err := createTestRepo(t, "RepoSearch1", c)
	assert.NoError(t, err)
	_, err = c.Repositories.AddRepoTopic(t.Context(), repo.Owner.UserName, repo.Name, "TestTopic1")
	assert.NoError(t, err)
	_, err = c.Repositories.AddRepoTopic(t.Context(), repo.Owner.UserName, repo.Name, "TestTopic2")
	assert.NoError(t, err)

	repo, err = createTestRepo(t, "RepoSearch2", c)
	assert.NoError(t, err)
	_, err = c.Repositories.AddRepoTopic(t.Context(), repo.Owner.UserName, repo.Name, "TestTopic1")
	assert.NoError(t, err)

	repos, _, err := c.Repositories.SearchRepos(t.Context(), gitea.SearchRepoOptions{
		Keyword:              "Search1",
		KeywordInDescription: true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, repos)
	assert.Len(t, repos, 1)

	repos, _, err = c.Repositories.SearchRepos(t.Context(), gitea.SearchRepoOptions{
		Keyword:              "Search",
		KeywordInDescription: true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, repos)
	assert.Len(t, repos, 2)

	repos, _, err = c.Repositories.SearchRepos(t.Context(), gitea.SearchRepoOptions{
		Keyword:              "TestTopic1",
		KeywordInDescription: true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, repos)
	assert.Len(t, repos, 2)

	repos, _, err = c.Repositories.SearchRepos(t.Context(), gitea.SearchRepoOptions{
		Keyword:              "TestTopic2",
		KeywordInDescription: true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, repos)
	assert.Len(t, repos, 1)

	_, err = c.Repositories.DeleteRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
}

func TestDeleteRepo(t *testing.T) {
	log.Println("== TestDeleteRepo ==")
	c := newTestClient()
	repo, _ := createTestRepo(t, "TestDeleteRepo", c)
	_, err := c.Repositories.DeleteRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
}

func TestGetArchive(t *testing.T) {
	log.Println("== TestGetArchive ==")
	c := newTestClient()
	repo, _ := createTestRepo(t, "ToDownload", c)
	time.Sleep(time.Second / 2)
	archive, _, err := c.Repositories.GetArchive(t.Context(), repo.Owner.UserName, repo.Name, "main", gitea.ZipArchive)
	assert.NoError(t, err)
	assert.True(t, len(archive) > 1500 && len(archive) < 1700)
}

func TestGetArchiveReader(t *testing.T) {
	log.Println("== TestGetArchiveReader ==")
	c := newTestClient()
	repo, _ := createTestRepo(t, "ToDownload", c)
	time.Sleep(time.Second / 2)
	r, _, err := c.Repositories.GetArchiveReader(t.Context(), repo.Owner.UserName, repo.Name, "main", gitea.ZipArchive)
	assert.NoError(t, err)
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("error when closing archive reader: %v", closeErr)
		}
	}()

	archive := bytes.NewBuffer(nil)
	nBytes, err := io.Copy(archive, r)
	assert.NoError(t, err)
	assert.True(t, nBytes > 1500)
	assert.EqualValues(t, nBytes, len(archive.Bytes()))
}

func TestGetRepoByID(t *testing.T) {
	log.Println("== TestGetRepoByID ==")
	c := newTestClient()
	testrepo, _ := createTestRepo(t, "TestGetRepoByID", c)

	repo, _, err := c.Repositories.GetRepoByID(t.Context(), testrepo.ID)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	assert.EqualValues(t, testrepo.ID, repo.ID)

	_, err = c.Repositories.DeleteRepo(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
}
