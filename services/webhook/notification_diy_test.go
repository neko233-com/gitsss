// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"strings"
	"testing"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationFriendlyAliases(t *testing.T) {
	payload := strings.Replace(richFixture, `"login":"alice"`, `"login":"alice","full_name":"Alice Remark"`, 1)
	payload = strings.Replace(payload, `"total_commits":12`, `"after":"`+strings.Repeat("a", 40)+`","total_commits":12`, 1)
	text, err := renderNotificationTemplate(`{{repository}} | {{branch}} | {{actor}} | {{actorName}} | {{actorDisplay}} | {{trackId}} | {{commitMessage}} | {{.Event}}`, webhook_module.HookEventPush, "", []byte(payload))
	require.NoError(t, err)
	assert.Contains(t, text, "engineering/demo | main | alice | Alice Remark | alice · Alice Remark | aaaaaaaaaaaa")
	assert.Contains(t, text, "Fix <script> & @everyone\nDetailed body | push")
	require.Error(t, ValidateNotificationTemplate(`{{unknownVariable}}`))
}

func TestNotificationJSONEscaping(t *testing.T) {
	values := notificationAliases("push", "", map[string]any{})
	values["commitMessage"] = "引号 \"\\\n</script> {{actor}}"
	for _, source := range []string{`{"text":"{{commitMessage}}"}`, `{"text":{{commitMessage}}}`, `{"text":{{json commitMessage}}}`} {
		payload, err := renderNotificationJSON(source, values)
		require.NoError(t, err)
		assert.Equal(t, values["commitMessage"], payload["text"])
	}
	payload, err := renderNotificationJSON(`{"text":"prefix {{commitMessage}} suffix"}`, values)
	require.NoError(t, err)
	assert.Equal(t, "prefix "+values["commitMessage"]+" suffix", payload["text"])
	for _, source := range []string{`{`, `[]`, `null`, `{}`, `{"text":{{unknown}}}`, `{"text":"{{if actor}}"}`, `{"text":"{{"}`, `{"text":"\{{actor}}"}`, strings.Repeat("x", MaxNotificationTemplateSize+1)} {
		_, err := renderNotificationJSON(source, values)
		require.Error(t, err, source)
	}
	values["commitMessage"] = strings.Repeat("x", maxNotificationOutputSize)
	_, err = renderNotificationJSON(`{"text":"{{commitMessage}}"}`, values)
	require.Error(t, err)
}

func TestNotificationJSONChannels(t *testing.T) {
	for _, test := range []struct {
		kind   webhook_module.HookType
		source string
	}{
		{webhook_module.EMAIL, `{"subject":"{{repository}}","text":"{{commitMessage}}"}`},
		{webhook_module.FEISHU, `{"msg_type":"interactive","card":{"header":{"template":"{{color}}","title":{"tag":"plain_text","content":"{{repository}}"}},"elements":[{"tag":"div","text":{"tag":"plain_text","content":"{{commitMessage}}"}}]},"sign":"untrusted","timestamp":1}`},
		{webhook_module.DISCORD, `{"content":"{{repository}}: {{commitMessage}}","allowed_mentions":{"parse":["everyone"]}}`},
		{webhook_module.TELEGRAM, `{"text":"{{repository}}: {{commitMessage}}","chat_id":"untrusted","message_thread_id":999}`},
		{webhook_module.WECHATWORK, `{"msgtype":"text","text":{"content":"{{repository}}: {{commitMessage}}"}}`},
		{webhook_module.DINGTALK, `{"msgtype":"text","text":{"content":"{{repository}}: {{commitMessage}}"}}`},
		{webhook_module.SLACK, `{"text":"{{repository}}: {{commitMessage}}","channel":"untrusted"}`},
		{webhook_module.MSTEAMS, `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","content":{"type":"AdaptiveCard","version":"1.4","body":[{"type":"TextBlock","text":"{{repository}}: {{commitMessage}}"}]}}]}`},
		{webhook_module.MATRIX, `{"msgtype":"m.notice","body":"{{repository}}: {{commitMessage}}"}`},
	} {
		t.Run(test.kind, func(t *testing.T) {
			settings, err := GetNotificationSettings("")
			require.NoError(t, err)
			settings.PayloadTemplate = test.source
			require.NoError(t, settings.ValidateForChannel(test.kind))
			meta, err := NotificationMeta(map[string]any{"channel": "#engineering", "recipients": "release@example.invalid"}, settings)
			require.NoError(t, err)
			updated, err := NotificationConfigMeta(test.kind, meta, map[string]string{})
			require.NoError(t, err)
			saved, err := GetNotificationSettings(updated)
			require.NoError(t, err)
			assert.Equal(t, test.source, saved.PayloadTemplate)
			hook := &webhook_model.Webhook{Type: test.kind, URL: "https://notify.example.com/endpoint?chat_id=123", Secret: "fixture-secret", Meta: meta}
			req, body, err := notificationRichRequest(t.Context(), hook, &webhook_model.HookTask{UUID: "delivery-id", EventType: webhook_module.HookEventPush, PayloadContent: richFixture})
			require.NoError(t, err)
			var rendered map[string]any
			require.NoError(t, json.Unmarshal(body, &rendered))
			assert.Contains(t, string(body), "engineering/demo")
			assert.Contains(t, string(body), "Detailed body")
			assert.Contains(t, req.URL.String(), "notify.example.com/endpoint")
			switch test.kind {
			case webhook_module.FEISHU:
				assert.NotEqual(t, "untrusted", rendered["sign"])
				assert.Equal(t, feishuGenSign(hook.Secret, int64(rendered["timestamp"].(float64))), rendered["sign"])
			case webhook_module.TELEGRAM:
				assert.NotContains(t, rendered, "chat_id")
				assert.NotContains(t, rendered, "message_thread_id")
				assert.Equal(t, "123", req.URL.Query().Get("chat_id"))
			case webhook_module.SLACK:
				assert.Equal(t, "#engineering", rendered["channel"])
			case webhook_module.DISCORD:
				assert.Empty(t, notificationMap(rendered["allowed_mentions"])["parse"])
			}
		})
	}
	settings := NotificationSettings{Style: "legacy", PayloadTemplate: `{"text":"{{commitMessage}}"}`}
	require.Error(t, settings.Validate())
	settings.Style = "rich"
	require.Error(t, settings.ValidateForChannel(webhook_module.FEISHU))
}
