// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
)

func Test_GCLFSConfig(t *testing.T) {
	cfg, err := setting.NewConfigProviderFromData(`
[cron.gc_lfs]
ENABLED = true
RUN_AT_START = true
SCHEDULE = "@every 2h"
OLDER_THAN = "1h"
LAST_UPDATED_MORE_THAN_AGO = "7h"
NUMBER_TO_CHECK_PER_REPO = 10
PROPORTION_TO_CHECK_PER_REPO = 0.1
`)
	assert.NoError(t, err)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()

	config := &GCLFSConfig{
		Enabled:                  false,
		RunAtStart:               false,
		Schedule:                 "@every 24h",
		OlderThan:                24 * time.Hour * 7,
		LastUpdatedMoreThanAgo:   24 * time.Hour * 3,
		NumberToCheckPerRepo:     100,
		ProportionToCheckPerRepo: 0.6,
	}

	_, err = setting.GetCronSettings("gc_lfs", config)
	assert.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.True(t, config.RunAtStart)
	assert.Equal(t, "@every 2h", config.Schedule)
	assert.Equal(t, 1*time.Hour, config.OlderThan)
	assert.Equal(t, 7*time.Hour, config.LastUpdatedMoreThanAgo)
	assert.Equal(t, int64(10), config.NumberToCheckPerRepo)
	assert.InDelta(t, 0.1, config.ProportionToCheckPerRepo, 0.001)
}

func TestUpdateCheckerConfig(t *testing.T) {
	cfg, err := setting.NewConfigProviderFromData(`
[cron.update_checker]
ENABLED = true
HTTP_ENDPOINT = http://updates.example/version.json
CHECK_GITHUB = true
GITHUB_REPO = neko233-com/gitsss
MIRRORS = https://mirror.example,https://backup.example/{url}
PROBE_TIMEOUT = 3s
`)
	assert.NoError(t, err)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()

	config := &UpdateCheckerConfig{
		Enabled: false, Schedule: "@every 168h",
	}
	_, err = setting.GetCronSettings("update_checker", config)
	assert.NoError(t, err)
	assert.True(t, config.Enabled)
	assert.True(t, config.CheckGitHub)
	assert.Equal(t, "neko233-com/gitsss", config.GitHubRepo)
	assert.Equal(t, 3*time.Second, config.ProbeTimeout)
	opts := config.options()
	assert.Equal(t, []string{"https://mirror.example", "https://backup.example/{url}"}, opts.Mirrors)
	assert.Equal(t, "http://updates.example/version.json", opts.HTTPEndpoint)
}
