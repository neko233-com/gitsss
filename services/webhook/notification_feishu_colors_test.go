// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeishuRepositoryColors(t *testing.T) {
	settings := NotificationSettings{}
	data := map[string]any{"repository": map[string]any{"id": float64(7)}}
	auto := notificationFeishuColor(settings, data, "engineering/server")
	assert.True(t, validFeishuColor(auto))
	assert.Equal(t, auto, notificationFeishuColor(settings, data, "engineering/renamed"), "repository ID preserves automatic color across renames")
	seen := map[string]bool{}
	for id := 1; id <= 200; id++ {
		data["repository"].(map[string]any)["id"] = float64(id)
		seen[notificationFeishuColor(settings, data, "engineering/server")] = true
	}
	assert.Len(t, seen, len(feishuHeaderColors))
	settings.FeishuHeaderColor = "blue"
	assert.Equal(t, "blue", notificationFeishuColor(settings, data, "engineering/server"))
	settings.FeishuRepositoryColors = "Engineering/Server = purple\r\nengineering/client = green\n"
	require.NoError(t, settings.Validate())
	assert.Equal(t, "purple", notificationFeishuColor(settings, data, "engineering/server"))
	assert.Equal(t, "green", notificationFeishuColor(settings, data, "ENGINEERING/CLIENT"))
	assert.Equal(t, "blue", notificationFeishuColor(settings, data, "other/repo"))
}

func TestFeishuRepositoryColorValidationAndCard(t *testing.T) {
	for _, color := range feishuHeaderColors {
		require.NoError(t, validateFeishuColors(color, ""))
	}
	for _, rules := range []string{"repo = green", "owner/repo = #123456", "owner/* = blue", "owner/repo = green\nOWNER/REPO = red", "owner/repo = <script>"} {
		assert.Error(t, validateFeishuColors("auto", rules), rules)
	}
	assert.Error(t, validateFeishuColors("invalid", ""))
	settings, err := GetNotificationSettings("")
	require.NoError(t, err)
	settings.FeishuRepositoryColors = "engineering/demo = orange"
	meta, err := NotificationMeta(map[string]any{"message_template": ""}, settings)
	require.NoError(t, err)
	updated, err := NotificationConfigMeta(webhook_module.FEISHU, meta, map[string]string{"message_template": ""})
	require.NoError(t, err)
	parsed, err := GetNotificationSettings(updated)
	require.NoError(t, err)
	assert.Equal(t, settings.FeishuRepositoryColors, parsed.FeishuRepositoryColors)
	data := feishuPushFixture(t)
	body, err := json.Marshal(data)
	require.NoError(t, err)
	task := &webhook_model.HookTask{PayloadContent: string(body), EventType: webhook_module.HookEventPush}
	message, err := buildNotificationMessage(settings, task)
	require.NoError(t, err)
	card := notificationFeishuCard(t.Context(), &webhook_model.Webhook{Meta: meta}, task, message)
	assert.Equal(t, "orange", notificationMap(notificationMap(card["card"])["header"])["template"])
}
