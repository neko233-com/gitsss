// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationChannelPolicies(t *testing.T) {
	stable := `{"action":"published","release":{"tag_name":"v1.2.3","draft":false,"prerelease":false}}`
	for _, test := range []struct {
		policy, tag string
		event       webhook_module.HookEventType
		payload     string
		match       bool
	}{
		{"all", "", webhook_module.HookEventPush, richFixture, true},
		{"formal_release", "v*", webhook_module.HookEventPush, richFixture, false},
		{"formal_release", "v*", webhook_module.HookEventRelease, stable, true},
		{"formal_release", "release-*", webhook_module.HookEventRelease, stable, false},
		{"formal_release", "v*", webhook_module.HookEventRelease, `{"action":"updated","release":{"tag_name":"v1.2.3","draft":false,"prerelease":false}}`, false},
		{"formal_release", "v*", webhook_module.HookEventRelease, `{"action":"published","release":{"tag_name":"v1.2.3","draft":true,"prerelease":false}}`, false},
		{"formal_release", "v*", webhook_module.HookEventRelease, `{"action":"published","release":{"tag_name":"v1.2.3","draft":false,"prerelease":true}}`, false},
		{"formal_release", "", webhook_module.HookEventRelease, `{"action":"published","release":{}}`, false},
		{"tag_created", "v*", webhook_module.HookEventCreate, `{"ref_type":"tag","ref":"v1.2.3"}`, true},
		{"tag_created", "v*", webhook_module.HookEventCreate, `{"ref_type":"branch","ref":"v1.2.3"}`, false},
		{"tag_created", "v*", webhook_module.HookEventPush, `{"ref":"refs/tags/v1.2.3"}`, false},
		{"tag_created", "v*", webhook_module.HookEventRelease, stable, false},
	} {
		assert.Equal(t, test.match, notificationEventMatches(NotificationSettings{EventPolicy: test.policy, TagFilter: test.tag}, test.event, []byte(test.payload)), "%+v", test)
	}
	require.Error(t, (NotificationSettings{EventPolicy: "unknown"}).Validate())
	require.Error(t, (NotificationSettings{TagFilter: "["}).Validate())
}

func TestNotificationEmailConfiguration(t *testing.T) {
	recipients, err := notificationEmailRecipients("Release <release@example.invalid>, release@example.invalid, owner@example.invalid")
	require.NoError(t, err)
	assert.Equal(t, []string{"release@example.invalid", "owner@example.invalid"}, recipients)
	for _, source := range []string{"", "not-an-address", "a@example.invalid\r\nBcc: b@example.invalid"} {
		_, err := notificationEmailRecipients(source)
		require.Error(t, err)
	}
	meta, err := NotificationConfigMeta(webhook_module.EMAIL, "", map[string]string{"recipients": "release@example.invalid", "notification": `{"event_policy":"formal_release","tag_filter":"v*","payload_template":"{\"subject\":\"{{repository}}\",\"text\":\"{{commitMessage}}\"}"}`})
	require.NoError(t, err)
	updated, err := NotificationConfigMeta(webhook_module.EMAIL, meta, map[string]string{"recipients": "owner@example.invalid"})
	require.NoError(t, err)
	settings, err := GetNotificationSettings(updated)
	require.NoError(t, err)
	assert.Equal(t, "formal_release", settings.EventPolicy)
	assert.Equal(t, "v*", settings.TagFilter)
	assert.Contains(t, settings.PayloadTemplate, "commitMessage")
	assert.Equal(t, "[configured mail transport]", notificationSafeURL(webhook_module.EMAIL, "mailto:configured-smtp"))
}
