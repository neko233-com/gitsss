// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"html"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/models/unittest"
	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/test"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fullMessageTask(t *testing.T, message string) *webhook_model.HookTask {
	t.Helper()
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(richFixture), &data))
	data["total_commits"] = 1
	data["after"] = strings.Repeat("a", 40)
	data["commits"].([]any)[0].(map[string]any)["message"] = message
	body, err := json.Marshal(data)
	require.NoError(t, err)
	return &webhook_model.HookTask{UUID: "multipart-fixture", EventType: webhook_module.HookEventPush, PayloadVersion: 2, PayloadContent: string(body)}
}

func notificationPartText(t *testing.T, kind webhook_module.HookType, body []byte) string {
	t.Helper()
	value := map[string]any{}
	require.NoError(t, json.Unmarshal(body, &value))
	switch kind {
	case webhook_module.EMAIL:
		return value["message"].(string)
	case webhook_module.FEISHU:
		return notificationMap(notificationMap(notificationMap(value["card"])["elements"].([]any)[2])["text"])["content"].(string)
	case webhook_module.DISCORD:
		return notificationMap(value["embeds"].([]any)[0])["description"].(string)
	case webhook_module.SLACK:
		return notificationMap(notificationMap(notificationMap(value["attachments"].([]any)[0])["blocks"].([]any)[2])["text"])["text"].(string)
	case webhook_module.MSTEAMS:
		return notificationMap(notificationMap(notificationMap(value["attachments"].([]any)[0])["content"])["body"].([]any)[2])["text"].(string)
	case webhook_module.TELEGRAM:
		text := value["text"].(string)
		_, text, _ = strings.Cut(text, "\n\n")
		text, _, _ = strings.Cut(text, "\n\n<i>")
		return html.UnescapeString(text)
	case webhook_module.WECHATWORK, webhook_module.DINGTALK:
		key := "content"
		if kind == webhook_module.DINGTALK {
			key = "text"
		}
		text := notificationMap(value["markdown"])[key].(string)
		_, text, _ = strings.Cut(text, "\n\n")
		text, _, _ = strings.Cut(text, "\n\nGitSSS")
		return text
	case webhook_module.MATRIX:
		text := value["body"].(string)
		_, text, _ = strings.Cut(text, "CI/CD · Actions: Unable to confirm\n")
		return text
	}
	t.Fatal("unknown channel")
	return ""
}

func TestNotificationFullMultilineAllChannels(t *testing.T) {
	message := strings.Repeat("第一行 文本🙂\n第二行 内容完整\n\n", 2000) + "最后一行 END"
	task := fullMessageTask(t, message)
	for _, kind := range []webhook_module.HookType{webhook_module.EMAIL, webhook_module.FEISHU, webhook_module.DISCORD, webhook_module.TELEGRAM, webhook_module.SLACK, webhook_module.WECHATWORK, webhook_module.DINGTALK, webhook_module.MSTEAMS, webhook_module.MATRIX} {
		t.Run(kind, func(t *testing.T) {
			hook := &webhook_model.Webhook{Type: kind, URL: "https://notify.example.com/hook", Meta: `{"notification":{"include_files":false,"max_commits":1,"include_commit_body":false}}`}
			parts, err := notificationRichRequests(t.Context(), hook, task)
			require.NoError(t, err)
			require.Greater(t, len(parts), 1)
			var reconstructed strings.Builder
			for index, part := range parts {
				require.True(t, notificationPayloadFits(kind, part.Body), "part %d", index)
				assert.Equal(t, strconv.Itoa(index+1), part.Request.Header.Get("X-GitSSS-Part"))
				reconstructed.WriteString(notificationPartText(t, kind, part.Body))
			}
			assert.Equal(t, message, reconstructed.String())
		})
	}
}

func TestNotificationPaginationTemplateEscaping(t *testing.T) {
	message := strings.Repeat("\n引号 \" \\ <script> {{actor}} 🙂", 2000)
	settings := NotificationSettings{PayloadTemplate: `{"msgtype":"m.notice","body":"{{commitMessage}}","page":"{{page}}/{{pages}}"}`}
	meta, err := NotificationMeta(nil, settings)
	require.NoError(t, err)
	parts, err := notificationRichRequests(t.Context(), &webhook_model.Webhook{Type: webhook_module.MATRIX, URL: "https://notify.example.com/room", Meta: meta}, fullMessageTask(t, message))
	require.NoError(t, err)
	var reconstructed strings.Builder
	seen := map[string]bool{}
	for _, part := range parts {
		var value map[string]any
		require.NoError(t, json.Unmarshal(part.Body, &value))
		reconstructed.WriteString(value["body"].(string))
		assert.False(t, seen[part.Request.URL.String()])
		seen[part.Request.URL.String()] = true
	}
	assert.Equal(t, message, reconstructed.String())
	_, err = notificationRichRequests(t.Context(), &webhook_model.Webhook{Type: webhook_module.FEISHU, URL: "https://notify.example.com", Meta: "{}"}, fullMessageTask(t, strings.Repeat("x", 1024*1024+1)))
	require.Error(t, err)
}

func TestNotificationMultipartStopsOnFailure(t *testing.T) {
	defer test.MockVariableValue(&notificationPartInterval, func(webhook_module.HookType) time.Duration { return 0 })()
	require.NoError(t, unittest.PrepareTestDatabase())
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 2 {
			_, _ = w.Write([]byte(`{"errcode":400,"errmsg":"fixture"}`))
		} else {
			_, _ = w.Write([]byte(`{"errcode":0}`))
		}
	}))
	defer server.Close()
	hook := &webhook_model.Webhook{RepoID: 3, IsActive: true, Type: webhook_module.WECHATWORK, URL: server.URL, Meta: "{}"}
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	task := fullMessageTask(t, strings.Repeat("多行消息\n", 2000))
	task.HookID = hook.ID
	task, err := webhook_model.CreateHookTask(t.Context(), task)
	require.NoError(t, err)
	require.Error(t, Deliver(t.Context(), task))
	assert.Equal(t, 2, calls)
	assert.False(t, task.IsSucceed)
	assert.Equal(t, "1", task.ResponseInfo.Headers["X-GitSSS-Parts-Accepted"])
	assert.Contains(t, task.ResponseInfo.Body, "part 2 failed")
}

func TestNotificationEmailPresentation(t *testing.T) {
	m := notificationMessage{Title: "代码更新", Repository: "engineering/demo", Branch: "main", Actor: "alice · 艾丽丝", TrackID: "aaaaaaaaaaaa", TrackURL: "https://git.example.com/commit/aaaa", URL: "https://git.example.com/changes", Body: "第一行\n\n<script>alert(1)</script>\n最后一行", ActionStatus: "已触发 · 排队中", Chinese: true}
	payload := notificationEmailPayload(m)
	body := notificationEmailHTML(payload, "fixture", payload["text"].(string))
	for _, value := range []string{"role=\"presentation\"", "white-space:pre-wrap", "分支 / 标签", "操作人", "trackId", "CI/CD", "查看变更", "alice · 艾丽丝", "&lt;script&gt;", "最后一行"} {
		assert.Contains(t, body, value)
	}
	assert.NotContains(t, body, "<script>")
	payload["url"] = "javascript:alert(1)"
	payload["trackUrl"] = "data:text/html,unsafe"
	assert.NotContains(t, notificationEmailHTML(payload, "fixture", "fixture"), "javascript:")
}
