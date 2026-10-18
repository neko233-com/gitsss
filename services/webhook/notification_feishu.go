// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/models/db"
	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

func notificationRepositoryURL(data map[string]any) string {
	repository := notificationMap(data["repository"])
	parts := strings.Split(notificationString(repository["full_name"]), "/")
	base := notificationURL(setting.AppURL)
	if base != "" && len(parts) == 2 && parts[0] != "" && parts[1] != "" && parts[0] != "." && parts[0] != ".." && parts[1] != "." && parts[1] != ".." {
		return strings.TrimRight(base, "/") + "/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
	}
	return notificationURL(repository["html_url"])
}

// API-generated payloads may contain a loopback request origin; shared cards use the configured site URL.
func notificationPublicRepositoryLink(data map[string]any, link string) string {
	original, err := url.Parse(notificationURL(notificationMap(data["repository"])["html_url"]))
	if err != nil || original.Path == "" {
		return link
	}
	target, err := url.Parse(link)
	if err != nil || (target.Path != original.Path && !strings.HasPrefix(target.Path, strings.TrimRight(original.Path, "/")+"/")) {
		return link
	}
	canonical, err := url.Parse(notificationRepositoryURL(data))
	if err != nil || canonical.Host == "" {
		return link
	}
	canonical.Path = strings.TrimRight(canonical.Path, "/") + strings.TrimPrefix(target.Path, strings.TrimRight(original.Path, "/"))
	canonical.RawPath = ""
	canonical.RawQuery, canonical.Fragment = target.RawQuery, target.Fragment
	return canonical.String()
}

func notificationActorDisplay(data map[string]any) string {
	login := notificationActor(data)
	for _, key := range []string{"sender", "pusher", "user"} {
		actor := notificationMap(data[key])
		if notificationActor(map[string]any{"sender": actor}) != login {
			continue
		}
		for _, field := range []string{"full_name", "name"} {
			if name := notificationString(actor[field]); name != "" && !strings.EqualFold(name, login) {
				return notificationLimit(login, 80) + " · " + notificationLimit(name, 80)
			}
		}
	}
	return notificationLimit(login, 160)
}

func notificationObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return strings.Trim(value, "0") != ""
}

func notificationTrack(data map[string]any, task *webhook_model.HookTask) (id, link string) {
	sha := notificationString(data["after"])
	if sha == "" {
		sha = notificationString(notificationMap(data["head_commit"])["id"])
		for _, key := range []string{"workflow_run", "workflow_job"} {
			if sha == "" {
				sha = notificationString(notificationMap(data[key])["head_sha"])
			}
		}
	}
	repoURL := notificationRepositoryURL(data)
	if notificationObjectID(sha) {
		if repoURL != "" {
			link = strings.TrimRight(repoURL, "/") + "/commit/" + sha
		}
		return sha[:12], link
	}
	// Deleted refs and incomplete legacy payloads have no new commit to link.
	return task.UUID, ""
}

func notificationMarkdownLink(label, link string) string {
	label = notificationMarkdown(label)
	if link == "" {
		return label
	}
	return "[" + label + "](" + strings.NewReplacer("(", "%28", ")", "%29").Replace(link) + ")"
}

func notificationPushRunMatches(data map[string]any, run *actions_model.ActionRun) bool {
	if run.Event != webhook_module.HookEventPush {
		return false
	}
	var recorded map[string]any
	if json.Unmarshal([]byte(run.EventPayload), &recorded) != nil {
		return false
	}
	for _, key := range []string{"before", "after", "ref"} {
		if notificationString(recorded[key]) != notificationString(data[key]) {
			return false
		}
	}
	return notificationActor(recorded) == notificationActor(data)
}

func notificationPushActions(ctx context.Context, data map[string]any, chinese bool) string {
	unknown := notificationLabel(chinese, "Unable to confirm", "暂时无法确认")
	repoID, _ := notificationMap(data["repository"])["id"].(float64)
	sha, ref := notificationString(data["after"]), notificationString(data["ref"])
	if repoID <= 0 || !notificationObjectID(sha) || ref == "" {
		return unknown
	}
	// Delivery can precede Actions detection; absence is a snapshot, never proof of a skipped trigger.
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	options := &actions_model.FindRunOptions{
		RepoID: int64(repoID), CommitSHA: sha, Ref: ref, TriggerEvent: webhook_module.HookEventPush,
	}
	options.Page, options.PageSize = 1, 20
	runs, err := db.Find[actions_model.ActionRun](queryCtx, options)
	if err != nil {
		return unknown
	}
	statuses := []string{}
	seen := map[actions_model.Status]bool{}
	for _, run := range runs {
		if !notificationPushRunMatches(data, run) || seen[run.Status] {
			continue
		}
		seen[run.Status] = true
		english, translated := "Status unknown", "状态未知"
		switch run.Status {
		case actions_model.StatusWaiting:
			english, translated = "Queued", "排队中"
		case actions_model.StatusRunning:
			english, translated = "Running", "运行中"
		case actions_model.StatusSuccess:
			english, translated = "Succeeded", "成功"
		case actions_model.StatusFailure:
			english, translated = "Failed", "失败"
		case actions_model.StatusCancelled:
			english, translated = "Cancelled", "已取消"
		case actions_model.StatusCancelling:
			english, translated = "Cancelling", "取消中"
		case actions_model.StatusSkipped:
			english, translated = "Skipped", "已跳过"
		case actions_model.StatusBlocked:
			english, translated = "Awaiting approval", "等待批准"
		}
		statuses = append(statuses, notificationLabel(chinese, english, translated))
	}
	if len(statuses) == 0 {
		return notificationLabel(chinese, "No run recorded at delivery", "发送时尚无运行记录")
	}
	return notificationLabel(chinese, "Triggered", "已触发") + " · " + strings.Join(statuses, " / ")
}

func notificationFeishuBody(m notificationMessage) string {
	if m.CustomBody || len(m.Commits) == 0 {
		return notificationFormattedBody(m, "markdown", 6000)
	}
	lines := make([]string, 0, len(m.Commits)+1)
	size := 0
	for _, commit := range m.Commits {
		line := notificationMarkdown(commit.Message)
		if size+len(line)+100 > 6000 {
			break
		}
		lines = append(lines, line)
		size += len(line) + 2
	}
	if m.Total > len(lines) {
		lines = append(lines, notificationLabel(m.Chinese, "Open changes for all messages", "查看变更以阅读全部消息"))
	}
	return strings.Join(lines, "\n\n")
}

func notificationFeishuCard(ctx context.Context, w *webhook_model.Webhook, task *webhook_model.HookTask, m notificationMessage) map[string]any {
	var data map[string]any
	_ = json.Unmarshal([]byte(task.PayloadContent), &data)
	m.URL = notificationPublicRepositoryLink(data, m.URL)
	workflow := notificationMap(data["workflow_run"])
	if workflow == nil {
		workflow = notificationMap(data["workflow_job"])
	}
	actorData := data
	if actor := notificationMap(workflow["trigger_actor"]); notificationString(actor["login"]) != "" {
		actorData = map[string]any{"sender": actor}
	}
	if m.Branch == "" {
		m.Branch = notificationString(workflow["head_branch"])
	}
	fields := []any{}
	addField := func(title, value string, short bool) {
		if value != "" {
			fields = append(fields, map[string]any{"is_short": short, "text": map[string]any{"tag": "lark_md", "content": "**" + title + "**\n" + value}})
		}
	}
	addField(notificationLabel(m.Chinese, "Repository", "仓库"), notificationMarkdown(m.Repository), true)
	addField(notificationLabel(m.Chinese, "Branch / tag", "分支 / 标签"), notificationMarkdown(m.Branch), true)
	addField(notificationLabel(m.Chinese, "Actor", "操作人"), notificationMarkdown(notificationActorDisplay(actorData)), true)
	if task.EventType == webhook_module.HookEventPush || workflow != nil {
		id, link := notificationTrack(data, task)
		addField("trackId", notificationMarkdownLink(id, link), true)
		var status string
		if workflow != nil {
			status = notificationString(workflow["conclusion"])
			if status == "" {
				status = notificationString(workflow["status"])
			}
			addField(notificationLabel(m.Chinese, "Trigger", "触发原因"), notificationMarkdown(notificationString(workflow["event"])), true)
			if attempt, ok := workflow["run_attempt"].(float64); ok {
				addField(notificationLabel(m.Chinese, "Attempt", "运行批次"), fmt.Sprintf("#%.0f", attempt), true)
			}
		} else {
			status = notificationPushActions(ctx, data, m.Chinese)
		}
		actionsURL := notificationRepositoryURL(data)
		if actionsURL != "" {
			actionsURL = strings.TrimRight(actionsURL, "/") + "/actions"
		}
		if workflow != nil {
			if link := notificationURL(workflow["html_url"]); link != "" {
				actionsURL = notificationPublicRepositoryLink(data, link)
			}
			if !m.CustomBody {
				m.Body = notificationString(workflow["display_title"])
				if m.Body == "" {
					m.Body = notificationString(workflow["name"])
				}
				if m.Body == "" {
					m.Body = m.Title
				}
			}
		}
		addField("CI/CD · Actions", notificationMarkdownLink(status, actionsURL), false)
	}
	elements := []any{
		map[string]any{"tag": "div", "fields": fields},
		map[string]any{"tag": "hr"},
		map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": notificationFeishuBody(m)}},
	}
	if m.URL != "" {
		elements = append(elements, map[string]any{"tag": "action", "actions": []any{map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": notificationLabel(m.Chinese, "Open changes", "查看变更")}, "type": "primary", "url": m.URL}}})
	}
	elements = append(elements, map[string]any{"tag": "note", "elements": []any{map[string]any{"tag": "plain_text", "content": m.Footer}}})
	value := map[string]any{"msg_type": "interactive", "card": map[string]any{"config": map[string]any{"wide_screen_mode": true}, "header": map[string]any{"template": "blue", "title": map[string]any{"tag": "plain_text", "content": notificationLimit(m.Title, 120)}}, "elements": elements}}
	if w.Secret != "" {
		now := time.Now().Unix()
		value["timestamp"], value["sign"] = now, feishuGenSign(w.Secret, now)
	}
	return value
}
