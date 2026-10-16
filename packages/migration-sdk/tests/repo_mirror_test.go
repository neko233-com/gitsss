// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestPushMirrors(t *testing.T) {
	log.Println("== TestPushMirrors ==")
	c := newTestClient()
	user, _, err := c.Users.GetMyUserInfo(t.Context())
	assert.NoError(t, err)

	repo, err := createTestRepo(t, "PushMirrorTest", c)
	assert.NoError(t, err)

	pm, _, err := c.Repositories.PushMirrors(t.Context(), user.UserName, repo.Name, gitea.CreatePushMirrorOption{
		RemoteAddress: "https://example.com/test/repo.git",
		Interval:      "8h0m0s",
	})
	assert.NoError(t, err)

	pms, _, err := c.Repositories.ListPushMirrors(t.Context(), user.UserName, repo.Name, gitea.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, pms, 1)
	assert.EqualValues(t, "https://example.com/test/repo.git", pms[0].RemoteAddress)

	pmGet, _, err := c.Repositories.GetPushMirrorByRemoteName(t.Context(), user.UserName, repo.Name, pms[0].RemoteName)
	assert.NoError(t, err)
	assert.EqualValues(t, pm.RemoteAddress, pmGet.RemoteAddress)

	_, err = c.Repositories.DeletePushMirror(t.Context(), user.UserName, repo.Name, pms[0].RemoteName)
	assert.NoError(t, err)

	_, _, err = c.Repositories.GetPushMirrorByRemoteName(t.Context(), user.UserName, repo.Name, pms[0].RemoteName)
	assert.Error(t, err)

	pms, _, err = c.Repositories.ListPushMirrors(t.Context(), user.UserName, repo.Name, gitea.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, pms, 0)
}
