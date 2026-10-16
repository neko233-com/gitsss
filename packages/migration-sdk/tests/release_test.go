// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"fmt"
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestRelease(t *testing.T) {
	log.Println("== TestRelease ==")
	c := newTestClient()
	publisher := mustCurrentUsername(t, c)

	repo, _ := createTestRepo(t, "ReleaseTests", c)

	// ListReleases
	rl, _, err := c.Releases.ListReleases(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListReleasesOptions{})
	assert.NoError(t, err)
	assert.Len(t, rl, 0)

	// CreateRelease
	r, _, err := c.Releases.CreateRelease(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateReleaseOption{
		TagName:      "awesome",
		Target:       "main",
		Title:        "Release 1",
		Note:         "yes it's awesome",
		IsDraft:      true,
		IsPrerelease: true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, "awesome", r.TagName)
	assert.EqualValues(t, true, r.IsPrerelease)
	assert.EqualValues(t, true, r.IsDraft)
	assert.EqualValues(t, "Release 1", r.Title)
	assert.EqualValues(t, fmt.Sprintf("%s/api/v1/repos/%s/releases/%d", getGiteaURL(), repo.FullName, r.ID), r.URL)
	assert.EqualValues(t, "main", r.Target)
	assert.EqualValues(t, "yes it's awesome", r.Note)
	assert.EqualValues(t, publisher, r.Publisher.UserName)
	rl, _, _ = c.Releases.ListReleases(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListReleasesOptions{})
	assert.Len(t, rl, 1)

	// GetRelease
	r2, _, err := c.Releases.GetRelease(t.Context(), repo.Owner.UserName, repo.Name, r.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, r, r2)
	r2, _, err = c.Releases.GetReleaseByTag(t.Context(), repo.Owner.UserName, repo.Name, r.TagName)
	assert.NoError(t, err)
	assert.EqualValues(t, r, r2)
	// ListRelease without pre-releases
	tr := true
	rl, _, err = c.Releases.ListReleases(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListReleasesOptions{
		IsPreRelease: &tr,
	})
	assert.NoError(t, err)
	assert.Len(t, rl, 1) // created release is a pre-release
	// EditRelease
	r2, _, err = c.Releases.EditRelease(t.Context(), repo.Owner.UserName, repo.Name, r.ID, gitea.EditReleaseOption{
		Title:        "Release Awesome",
		Note:         "",
		IsDraft:      gitea.OptionalBool(false),
		IsPrerelease: gitea.OptionalBool(false),
	})
	assert.NoError(t, err)
	assert.EqualValues(t, r.Target, r2.Target)
	assert.EqualValues(t, false, r2.IsDraft)
	assert.EqualValues(t, false, r2.IsPrerelease)
	assert.EqualValues(t, r.Note, r2.Note)

	// GetLatestRelease
	r3, _, err := c.Releases.GetLatestRelease(t.Context(), repo.Owner.UserName, repo.Name)
	assert.NoError(t, err)
	assert.EqualValues(t, r2, r3)

	// DeleteRelease
	_, err = c.Releases.DeleteRelease(t.Context(), repo.Owner.UserName, repo.Name, r.ID)
	assert.NoError(t, err)
	rl, _, _ = c.Releases.ListReleases(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListReleasesOptions{})
	assert.Len(t, rl, 0)

	// CreateRelease
	_, _, err = c.Releases.CreateRelease(t.Context(), repo.Owner.UserName, repo.Name, gitea.CreateReleaseOption{
		TagName: "aNewReleaseTag",
		Target:  "main",
		Title:   "Title of aNewReleaseTag",
	})
	assert.NoError(t, err)

	// DeleteReleaseByTag
	_, err = c.Releases.DeleteReleaseByTag(t.Context(), repo.Owner.UserName, repo.Name, "aNewReleaseTag")
	assert.NoError(t, err)
	rl, _, _ = c.Releases.ListReleases(t.Context(), repo.Owner.UserName, repo.Name, gitea.ListReleasesOptions{})
	assert.Len(t, rl, 0)
	_, err = c.Releases.DeleteReleaseByTag(t.Context(), repo.Owner.UserName, repo.Name, "aNewReleaseTag")
	assert.Error(t, err)

	// Test Response if try to get not existing release
	_, resp, err := c.Releases.GetRelease(t.Context(), repo.Owner.UserName, repo.Name, 1234)
	assert.Error(t, err)
	if assert.NotNil(t, resp) {
		assert.EqualValues(t, 404, resp.StatusCode)
	}
	_, resp, err = c.Releases.GetReleaseByTag(t.Context(), repo.Owner.UserName, repo.Name, "not_here")
	assert.Error(t, err)
	if assert.NotNil(t, resp) {
		assert.EqualValues(t, 404, resp.StatusCode)
	}
}
