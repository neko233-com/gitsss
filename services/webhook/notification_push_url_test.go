// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationPushDestinations(t *testing.T) {
	repo := "https://git.example.com/engineering/demo"
	sha := strings.Repeat("a", 40)
	assert.Equal(t, repo+"/commit/"+sha, notificationPushURL(map[string]any{"after": sha, "before": strings.Repeat("0", 40), "compare_url": "https://git.example.com/"}, repo))
	compare := repo + "/compare/" + strings.Repeat("b", 40) + "..." + sha
	assert.Equal(t, compare, notificationPushURL(map[string]any{"after": sha, "before": strings.Repeat("b", 40), "compare_url": compare}, repo))
	assert.Equal(t, repo, notificationPushURL(map[string]any{"after": strings.Repeat("0", 40), "compare_url": "https://git.example.com/"}, repo))
	assert.Equal(t, repo, notificationPushURL(map[string]any{"compare_url": repo + "/compare/" + strings.Repeat("0", 40) + "..." + sha}, repo))
}
