// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushMirrors(t *testing.T) {
	log.Println("== TestPushMirrors ==")
	c := newTestClient()
	user, _, err := c.GetMyUserInfo()
	assert.NoError(t, err)

	repo, err := createTestRepo(t, "PushMirrorTest", c)
	assert.NoError(t, err)

	pm, _, err := c.PushMirrors(user.UserName, repo.Name, CreatePushMirrorOption{
		RemoteAddress: "https://example.com/test/repo.git",
		Interval:      "8h0m0s",
	})
	assert.NoError(t, err)

	pms, _, err := c.ListPushMirrors(user.UserName, repo.Name, ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, pms, 1)
	assert.EqualValues(t, "https://example.com/test/repo.git", pms[0].RemoteAddress)

	pmGet, _, err := c.GetPushMirrorByRemoteName(user.UserName, repo.Name, pms[0].RemoteName)
	assert.NoError(t, err)
	assert.EqualValues(t, pm.RemoteAddress, pmGet.RemoteAddress)

	_, err = c.DeletePushMirror(user.UserName, repo.Name, pms[0].RemoteName)
	assert.NoError(t, err)

	_, _, err = c.GetPushMirrorByRemoteName(user.UserName, repo.Name, pms[0].RemoteName)
	assert.Error(t, err)

	pms, _, err = c.ListPushMirrors(user.UserName, repo.Name, ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, pms, 0)
}
