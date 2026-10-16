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

func TestGetGlobalSettings(t *testing.T) {
	log.Println("== TestGetGlobalSettings ==")
	c := newTestClient()

	uiSettings, _, err := c.Settings.GetGlobalUISettings(t.Context())
	assert.NoError(t, err)
	expectedAllowedReactions := []string{"+1", "-1", "laugh", "hooray", "confused", "heart", "rocket", "eyes"}
	assert.ElementsMatch(t, expectedAllowedReactions, uiSettings.AllowedReactions)

	repoSettings, _, err := c.Settings.GetGlobalRepoSettings(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, &gitea.GlobalRepoSettings{
		HTTPGitDisabled: false,
		MirrorsDisabled: false,
		LFSDisabled:     true,
	}, repoSettings)

	apiSettings, _, err := c.Settings.GetGlobalAPISettings(t.Context())
	assert.NoError(t, err)
	assert.EqualValues(t, &gitea.GlobalAPISettings{
		MaxResponseItems:       50,
		DefaultPagingNum:       30,
		DefaultGitTreesPerPage: 1000,
		DefaultMaxBlobSize:     10485760,
	}, apiSettings)

	attachSettings, _, err := c.Settings.GetGlobalAttachmentSettings(t.Context())
	assert.NoError(t, err)
	if assert.NotEmpty(t, attachSettings.AllowedTypes) {
		attachSettings.AllowedTypes = ""
	}
	assert.EqualValues(t, &gitea.GlobalAttachmentSettings{
		Enabled:  true,
		MaxSize:  100,
		MaxFiles: 5,
	}, attachSettings)
}
