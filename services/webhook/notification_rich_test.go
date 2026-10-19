// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"
	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const richFixture = `{"repository":{"full_name":"engineering/demo","html_url":"https://git.example.com/engineering/demo"},"ref":"refs/heads/main","sender":{"login":"alice"},"total_commits":12,"compare_url":"https://git.example.com/compare/old...new","commits":[{"id":"abcdef1234567890","message":"Fix <script> & @everyone\nDetailed body","url":"https://git.example.com/commit/abcdef","author":{"name":"Alice"},"added":["docs/index.html"]}]}`

func TestNotificationRichChannels(t *testing.T) {
	for _, kind := range []webhook_module.HookType{webhook_module.FEISHU, webhook_module.TELEGRAM, webhook_module.DISCORD, webhook_module.WECHATWORK, webhook_module.DINGTALK, webhook_module.SLACK, webhook_module.MSTEAMS, webhook_module.MATRIX} {
		t.Run(kind, func(t *testing.T) {
			settings, err := GetNotificationSettings("")
			require.NoError(t, err)
			settings.Language = "zh-CN"
			meta, err := NotificationMeta(map[string]any{"chat_id": "123", "channel": "#engineering"}, settings)
			require.NoError(t, err)
			req, body, err := notificationRichRequest(t.Context(), &webhook_model.Webhook{Type: kind, URL: "https://notify.example.com/endpoint?chat_id=123", Meta: meta}, &webhook_model.HookTask{EventType: webhook_module.HookEventPush, PayloadContent: richFixture})
			require.NoError(t, err)
			require.NotNil(t, req)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			assert.Contains(t, string(body), "engineering/demo")
			assert.Contains(t, string(body), "main")
			assert.Contains(t, string(body), "alice")
			assert.Contains(t, string(body), "Detailed body")
			if kind != webhook_module.FEISHU {
				assert.Contains(t, string(body), "docs/index.html")
				assert.Contains(t, string(body), "11")
			}
			assert.Less(t, len(body), 28000)
			if kind == webhook_module.FEISHU {
				assert.Equal(t, "interactive", payload["msg_type"])
			}
			if kind == webhook_module.TELEGRAM {
				assert.Equal(t, "HTML", payload["parse_mode"])
				_, err := xml.NewDecoder(strings.NewReader("<root>" + payload["text"].(string) + "</root>")).Token()
				require.NoError(t, err)
				assert.NotContains(t, payload["text"], "<script>")
			}
			if kind == webhook_module.DISCORD {
				assert.Empty(t, notificationMap(payload["allowed_mentions"])["parse"])
			}
			if kind == webhook_module.WECHATWORK {
				assert.LessOrEqual(t, len(notificationMap(payload["markdown"])["content"].(string)), 4096)
			}
			if kind == webhook_module.MATRIX {
				assert.Equal(t, "PUT", req.Method)
			}
		})
	}
}

func TestNotificationDeliveryRecordsProviderFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":19024,"msg":"fixture rejection"}`))
	}))
	t.Cleanup(server.Close)
	hook := &webhook_model.Webhook{RepoID: 3, IsActive: true, Type: webhook_module.FEISHU, URL: server.URL + "/fixture-secret", Meta: "{}"}
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	task, err := webhook_model.CreateHookTask(t.Context(), &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPush, PayloadContent: richFixture, PayloadVersion: 2})
	require.NoError(t, err)
	require.Error(t, Deliver(t.Context(), task))
	assert.True(t, task.IsDelivered)
	assert.False(t, task.IsSucceed)
	assert.Equal(t, http.StatusOK, task.ResponseInfo.Status)
	assert.NotContains(t, task.RequestInfo.URL, "fixture-secret")
	assert.Contains(t, task.RequestInfo.Body, `"interactive"`)
}

func TestNotificationRoutingAndConfiguration(t *testing.T) {
	settings, err := GetNotificationSettings("")
	require.NoError(t, err)
	settings.RepositoryFilter = "engineering/*"
	settings.ActorFilter = "{alice,bob}"
	match, err := NotificationMatches(settings, []byte(richFixture))
	require.NoError(t, err)
	assert.True(t, match)
	settings.ExcludeRepository = "*/demo"
	match, err = NotificationMatches(settings, []byte(richFixture))
	require.NoError(t, err)
	assert.False(t, match)
	settings.ExcludeRepository = ""
	settings.SkipBots = true
	match, err = NotificationMatches(settings, []byte(strings.ReplaceAll(richFixture, `"alice"`, `"alice[bot]"`)))
	require.NoError(t, err)
	assert.False(t, match)
	meta, err := NotificationConfigMeta(webhook_module.TELEGRAM, `{"bot_token":"fixture-secret","chat_id":"123"}`, map[string]string{"notification": `{"language":"zh-CN","max_commits":3}`})
	require.NoError(t, err)
	assert.Contains(t, meta, "fixture-secret")
	assert.Contains(t, meta, "123")
	config, err := GetNotificationSettings(meta)
	require.NoError(t, err)
	assert.Equal(t, 3, config.MaxCommits)
	assert.True(t, config.IncludeFiles)
	_, err = NotificationConfigMeta(webhook_module.FEISHU, "", map[string]string{"notification": `{"color":"red"}`})
	assert.Error(t, err)
	assert.Equal(t, "https://notify.example.com/[redacted]", notificationSafeURL(webhook_module.FEISHU, "https://notify.example.com/hook/fixture-secret?key=secret"))
}

func TestNotificationProviderFailures(t *testing.T) {
	for _, test := range []struct {
		kind webhook_module.HookType
		body string
		fail bool
	}{
		{webhook_module.FEISHU, `{"code":0}`, false},
		{webhook_module.FEISHU, `{"code":19024}`, true},
		{webhook_module.FEISHU, `{"StatusCode":100}`, true},
		{webhook_module.WECHATWORK, `{"errcode":40058}`, true},
		{webhook_module.TELEGRAM, `{"ok":false}`, true},
		{webhook_module.TELEGRAM, `{"ok":true}`, false},
		{webhook_module.MATRIX, `{"errcode":"M_FORBIDDEN"}`, true},
		{webhook_module.SLACK, `ok`, false},
		{webhook_module.DISCORD, ``, false},
	} {
		assert.Equal(t, test.fail, notificationResponseError(test.kind, []byte(test.body)) != nil, "%s %s", test.kind, test.body)
	}
}

func TestNotificationTemplateBoundedWriter(t *testing.T) {
	_, err := renderNotificationTemplate(`{{range .Payload.items}}{{.}}{{end}}`, webhook_module.HookEventPush, "", []byte(`{"items":["`+strings.Repeat("x", 40000)+`","`+strings.Repeat("y", 40000)+`"]}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "64 KiB")
}
