// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"strings"
	"testing"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/unittest"
	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func feishuPushFixture(t *testing.T) map[string]any {
	t.Helper()
	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://git.example.com/"))
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(richFixture), &data))
	data["before"], data["after"] = strings.Repeat("b", 40), strings.Repeat("a", 40)
	data["sender"] = map[string]any{"login": "alice", "full_name": "艾丽丝 @everyone"}
	return data
}

func TestFeishuPublicLinksFromLoopbackPayload(t *testing.T) {
	data := feishuPushFixture(t)
	t.Cleanup(test.MockVariableValue(&setting.AppURL, "https://git.example.com/gitsss/"))
	notificationMap(data["repository"])["html_url"] = "http://127.0.0.1:3000/engineering/demo"
	_, track := notificationTrack(data, &webhook_model.HookTask{})
	assert.Equal(t, "https://git.example.com/gitsss/engineering/demo/commit/"+strings.Repeat("a", 40), track)
	for _, path := range []string{"/actions", "/actions/runs/9", "/compare/aaa...bbb?expand=1#diff-1"} {
		assert.Equal(t, "https://git.example.com/gitsss/engineering/demo"+path, notificationPublicRepositoryLink(data, "http://127.0.0.1:3000/engineering/demo"+path))
	}
	assert.Equal(t, "https://external.example/status", notificationPublicRepositoryLink(data, "https://external.example/status"))
}

func TestFeishuConcisePushCard(t *testing.T) {
	data := feishuPushFixture(t)
	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	settings, err := GetNotificationSettings("")
	require.NoError(t, err)
	settings.Language = "zh-CN"
	task := &webhook_model.HookTask{EventType: webhook_module.HookEventPush, PayloadContent: string(encoded), UUID: "delivery-fixture"}
	m, err := buildNotificationMessage(settings, task)
	require.NoError(t, err)
	card := notificationFeishuCard(t.Context(), &webhook_model.Webhook{}, task, m)
	content := notificationMap(card["card"])
	assert.Equal(t, "代码更新", notificationMap(notificationMap(content["header"])["title"])["content"])
	elements := content["elements"].([]any)
	fields := notificationMap(elements[0])["fields"].([]any)
	require.Len(t, fields, 5)
	assert.Equal(t, "**操作人**\nalice · 艾丽丝 ＠everyone", notificationMap(notificationMap(fields[2])["text"])["content"])
	assert.Equal(t, "**trackId**\n[aaaaaaaaaaaa](https://git.example.com/engineering/demo/commit/"+strings.Repeat("a", 40)+")", notificationMap(notificationMap(fields[3])["text"])["content"])
	body := notificationMap(notificationMap(elements[2])["text"])["content"].(string)
	assert.Contains(t, body, "Fix &lt;script&gt; & ＠everyone")
	for _, omitted := range []string{"abcdef", "Alice", "docs/index.html", "提交数", "—", "11"} {
		assert.NotContains(t, body, omitted)
	}
	assert.NotContains(t, body, "Detailed body")
	assert.Contains(t, notificationMap(notificationMap(fields[4])["text"])["content"], "暂时无法确认")
}

func TestNotificationActorDisplay(t *testing.T) {
	for _, test := range []struct{ payload, expected string }{
		{`{"sender":{"login":"alice","full_name":"Alice"}}`, "alice"},
		{`{"sender":{"login":"alice"},"pusher":{"login":"alice","full_name":"艾丽丝"}}`, "alice · 艾丽丝"},
		{`{"sender":{"login":"alice"},"pusher":{"login":"bob","full_name":"Bob Remark"}}`, "alice"},
		{`{"sender":{"login":"alice","full_name":"艾丽丝"}}`, "alice · 艾丽丝"},
	} {
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(test.payload), &data))
		assert.Equal(t, test.expected, notificationActorDisplay(data))
		assert.Equal(t, "alice", notificationActor(data), "routing continues to use login only")
	}
}

func TestNotificationTrack(t *testing.T) {
	task := &webhook_model.HookTask{UUID: "delivery-fixture"}
	data := feishuPushFixture(t)
	for _, length := range []int{40, 64} {
		data["after"] = strings.Repeat("a", length)
		id, link := notificationTrack(data, task)
		assert.Equal(t, "aaaaaaaaaaaa", id)
		assert.Equal(t, "https://git.example.com/engineering/demo/commit/"+strings.Repeat("a", length), link)
	}
	data["after"] = strings.Repeat("0", 40)
	id, link := notificationTrack(data, task)
	assert.Equal(t, task.UUID, id)
	assert.Empty(t, link)
	data["after"] = ""
	data["head_commit"] = map[string]any{"id": strings.Repeat("c", 40)}
	id, link = notificationTrack(data, task)
	assert.Equal(t, "cccccccccccc", id)
	assert.Contains(t, link, "/commit/"+strings.Repeat("c", 40))
}

func TestNotificationPushActionsSnapshot(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	data := feishuPushFixture(t)
	notificationMap(data["repository"])["id"] = float64(4)
	assert.Equal(t, "发送时尚无运行记录", notificationPushActions(t.Context(), data, true))
	body, err := json.Marshal(data)
	require.NoError(t, err)
	run := &actions_model.ActionRun{RepoID: 4, Index: 999, CommitSHA: data["after"].(string), Ref: "refs/heads/main", Event: webhook_module.HookEventPush, TriggerEvent: "push", EventPayload: string(body), Status: actions_model.StatusWaiting}
	require.NoError(t, db.Insert(t.Context(), run))
	assert.Equal(t, "已触发 · 排队中", notificationPushActions(t.Context(), data, true))
	run.Status = actions_model.StatusSuccess
	require.NoError(t, actions_model.UpdateRun(t.Context(), run, "status"))
	assert.Equal(t, "Triggered · Succeeded", notificationPushActions(t.Context(), data, false))
	data["before"] = strings.Repeat("c", 40)
	assert.Equal(t, "发送时尚无运行记录", notificationPushActions(t.Context(), data, true), "another push of the same SHA is not this event")
	data["before"] = strings.Repeat("b", 40)
	data["ref"] = "refs/heads/other"
	assert.Equal(t, "发送时尚无运行记录", notificationPushActions(t.Context(), data, true))
	data["ref"] = "refs/heads/main"
	data["sender"] = map[string]any{"login": "bob"}
	assert.Equal(t, "发送时尚无运行记录", notificationPushActions(t.Context(), data, true))
}

func TestFeishuWorkflowTrace(t *testing.T) {
	data := feishuPushFixture(t)
	delete(data, "after")
	delete(data, "ref")
	data["workflow_run"] = map[string]any{"head_sha": strings.Repeat("c", 40), "head_branch": "main", "event": "push", "run_attempt": 2, "status": "completed", "conclusion": "success", "display_title": "Build package", "html_url": "https://git.example.com/engineering/demo/actions/runs/9", "trigger_actor": map[string]any{"login": "bob", "full_name": "鲍勃"}}
	body, err := json.Marshal(data)
	require.NoError(t, err)
	settings, err := GetNotificationSettings("")
	require.NoError(t, err)
	settings.Language = "zh-CN"
	task := &webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowRun, PayloadContent: string(body)}
	m, err := buildNotificationMessage(settings, task)
	require.NoError(t, err)
	payload := notificationFeishuCard(t.Context(), &webhook_model.Webhook{}, task, m)
	rendered, err := json.Marshal(payload)
	require.NoError(t, err)
	for _, expected := range []string{"bob · 鲍勃", "cccccccccccc", "/commit/" + strings.Repeat("c", 40), "触发原因", "push", "运行批次", "#2", "success", "Build package"} {
		assert.Contains(t, string(rendered), expected)
	}
	assert.NotContains(t, string(rendered), "提交数")
}
