// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/glob"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

// NotificationSettings is stored inside existing webhook metadata; routing and delivery IDs stay unchanged.
type NotificationSettings struct {
	Style             string `json:"style,omitempty"`
	Language          string `json:"language,omitempty"`
	TitleTemplate     string `json:"title_template,omitempty"`
	MessageTemplate   string `json:"message_template,omitempty"`
	Footer            string `json:"footer,omitempty"`
	Color             string `json:"color,omitempty"`
	MaxCommits        int    `json:"max_commits,omitempty"`
	IncludeFiles      bool   `json:"include_files"`
	IncludeCommitBody bool   `json:"include_commit_body"`
	RepositoryFilter  string `json:"repository_filter,omitempty"`
	ExcludeRepository string `json:"exclude_repository,omitempty"`
	ActorFilter       string `json:"actor_filter,omitempty"`
	SkipBots          bool   `json:"skip_bots"`
}

func NotificationWebhookSupported(kind webhook_module.HookType) bool {
	switch kind {
	case webhook_module.FEISHU, webhook_module.WECHATWORK, webhook_module.TELEGRAM,
		webhook_module.DISCORD, webhook_module.SLACK, webhook_module.DINGTALK,
		webhook_module.MSTEAMS, webhook_module.MATRIX:
		return true
	}
	return false
}

func GetNotificationSettings(meta string) (NotificationSettings, error) {
	value := struct {
		Notification NotificationSettings `json:"notification"`
	}{Notification: NotificationSettings{Style: "rich", Language: "en-US", MaxCommits: 5, IncludeFiles: true}}
	if strings.TrimSpace(meta) != "" {
		if err := json.Unmarshal([]byte(meta), &value); err != nil {
			return value.Notification, err
		}
	}
	if value.Notification.Style == "" {
		value.Notification.Style = "rich"
	}
	if value.Notification.MaxCommits == 0 {
		value.Notification.MaxCommits = 5
	}
	return value.Notification, value.Notification.Validate()
}

func (n NotificationSettings) Validate() error {
	if n.Style != "" && n.Style != "rich" && n.Style != "legacy" {
		return errors.New("unknown notification presentation")
	}
	if n.Language != "" && n.Language != "en-US" && n.Language != "zh-CN" {
		return errors.New("unsupported notification language")
	}
	if n.MaxCommits < 0 || n.MaxCommits > 10 || len(n.Footer) > 300 || len(n.Color) > 7 {
		return errors.New("notification detail limits exceeded")
	}
	if n.Color != "" && !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(n.Color) {
		return errors.New("notification color must use #RRGGBB")
	}
	for _, pattern := range []string{n.RepositoryFilter, n.ExcludeRepository, n.ActorFilter} {
		if len(pattern) > 512 {
			return errors.New("notification filter exceeds 512 bytes")
		}
		if pattern != "" {
			if _, err := glob.Compile(pattern); err != nil {
				return fmt.Errorf("invalid notification glob: %w", err)
			}
		}
	}
	for _, source := range []string{n.TitleTemplate, n.MessageTemplate} {
		if err := ValidateNotificationTemplate(source); err != nil {
			return err
		}
	}
	return nil
}

func NotificationMeta(meta any, settings NotificationSettings) (string, error) {
	if err := settings.Validate(); err != nil {
		return "", err
	}
	fields := map[string]any{}
	if meta != nil {
		body, err := json.Marshal(meta)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(body, &fields); err != nil {
			return "", err
		}
	}
	fields["notification"] = settings
	body, err := json.Marshal(fields)
	return string(body), err
}

func notificationMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func notificationString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func notificationActor(payload map[string]any) string {
	for _, key := range []string{"sender", "pusher", "user"} {
		actor := notificationMap(payload[key])
		for _, field := range []string{"login", "username", "name"} {
			if name := notificationString(actor[field]); name != "" {
				return name
			}
		}
	}
	return "GitSSS"
}

func NotificationMatches(settings NotificationSettings, payloadJSON []byte) (bool, error) {
	payload := map[string]any{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return false, err
	}
	name := notificationString(notificationMap(payload["repository"])["full_name"])
	actor := notificationActor(payload)
	for _, rule := range []struct {
		pattern, value string
		exclude        bool
	}{{settings.RepositoryFilter, name, false}, {settings.ExcludeRepository, name, true}, {settings.ActorFilter, actor, false}} {
		if rule.pattern == "" {
			continue
		}
		matcher, err := glob.Compile(rule.pattern)
		if err != nil {
			return false, err
		}
		if matcher.Match(rule.value) == rule.exclude {
			return false, nil
		}
	}
	actorType := notificationString(notificationMap(payload["sender"])["type"])
	return !settings.SkipBots || (!strings.HasSuffix(strings.ToLower(actor), "[bot]") && !strings.EqualFold(actorType, "bot")), nil
}

type notificationCommit struct {
	SHA, URL, Message, Author, Files string
}

type notificationMessage struct {
	Title, Repository, Branch, Actor, URL, Body, Footer string
	Commits                                             []notificationCommit
	Total                                               int
	More                                                int
	Chinese                                             bool
	CustomBody                                          bool
}

func notificationLimit(text string, count int) string {
	runes := []rune(text)
	if len(runes) > count {
		return string(runes[:count-1]) + "…"
	}
	return text
}

func notificationBytes(text string, count int) string {
	if count < 4 {
		return ""
	}
	if len(text) <= count {
		return text
	}
	end := count - len("…")
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end] + "…"
}

func notificationURL(value any) string {
	text := notificationString(value)
	u, err := url.Parse(text)
	if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || len(text) > 2048 {
		return ""
	}
	return text
}

func notificationMarkdown(text string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "@", "＠").Replace(text)
}

func notificationLabel(chinese bool, english, translated string) string {
	if chinese {
		return translated
	}
	return english
}

func buildNotificationMessage(settings NotificationSettings, t *webhook_model.HookTask) (notificationMessage, error) {
	data := map[string]any{}
	if err := json.Unmarshal([]byte(t.PayloadContent), &data); err != nil {
		return notificationMessage{}, err
	}
	repository := notificationMap(data["repository"])
	m := notificationMessage{Repository: notificationString(repository["full_name"]), Actor: notificationActor(data), URL: notificationURL(repository["html_url"]), Footer: settings.Footer, Chinese: settings.Language == "zh-CN"}
	m.Branch = strings.TrimPrefix(strings.TrimPrefix(notificationString(data["ref"]), "refs/heads/"), "refs/tags/")
	if m.Repository == "" {
		m.Repository = "GitSSS"
	}
	if m.Footer == "" {
		m.Footer = "GitSSS · " + notificationLabel(m.Chinese, "Engineering notifications", "研发协作通知")
	}
	m.Title = strings.ReplaceAll(string(t.EventType), "_", " ")
	for _, event := range []struct{ kind, english, chinese string }{
		{"issues", "Issue updated", "工单更新"},
		{"issue_comment", "New discussion", "讨论更新"},
		{"pull_request", "Pull request updated", "合并请求更新"},
		{"pull_request_review_approved", "Review approved", "审查通过"},
		{"pull_request_review_rejected", "Changes requested", "请求修改"},
		{"pull_request_review_comment", "Review comment", "代码审查评论"},
		{"release", "Release published", "版本发布"},
		{"workflow_run", "Workflow run", "工作流运行"},
		{"workflow_job", "Workflow job", "工作流任务"},
		{"status", "Commit status", "提交状态"},
		{"create", "Branch or tag created", "新建分支或标签"},
		{"delete", "Branch or tag deleted", "删除分支或标签"},
		{"wiki", "Wiki updated", "百科更新"},
		{"package", "Package updated", "软件包更新"},
		{"repository", "Repository updated", "仓库更新"},
	} {
		if event.kind == string(t.EventType) {
			m.Title = notificationLabel(m.Chinese, event.english, event.chinese)
			break
		}
	}
	if t.EventType == webhook_module.HookEventPush {
		m.Title = notificationLabel(m.Chinese, "Code updated", "代码更新")
		m.URL = notificationPushURL(data, m.URL)
		commits, _ := data["commits"].([]any)
		m.Total = len(commits)
		if count, ok := data["total_commits"].(float64); ok && int(count) > m.Total {
			m.Total = int(count)
		}
		for _, value := range commits {
			if len(m.Commits) >= settings.MaxCommits {
				break
			}
			row := notificationMap(value)
			message := notificationString(row["message"])
			if !settings.IncludeCommitBody {
				message = strings.SplitN(message, "\n", 2)[0]
			}
			commit := notificationCommit{SHA: notificationString(row["id"]), URL: notificationURL(row["url"]), Message: notificationLimit(message, 300), Author: notificationLimit(notificationString(notificationMap(row["author"])["name"]), 80)}
			if len(commit.SHA) > 12 {
				commit.SHA = commit.SHA[:12]
			}
			if settings.IncludeFiles {
				files := []string{}
				for _, change := range []struct{ key, prefix string }{{"added", "+"}, {"modified", "~"}, {"removed", "−"}} {
					items, _ := row[change.key].([]any)
					for _, value := range items {
						if len(files) < 5 {
							files = append(files, change.prefix+notificationLimit(notificationString(value), 90))
						}
					}
				}
				commit.Files = strings.Join(files, " · ")
			}
			m.Commits = append(m.Commits, commit)
		}
		m.More = m.Total - len(m.Commits)
	} else {
		for _, key := range []string{"comment", "review", "pull_request", "issue", "release", "workflow_run", "workflow_job", "package"} {
			item := notificationMap(data[key])
			if item == nil {
				continue
			}
			title := notificationString(item["title"])
			if title == "" {
				title = notificationString(item["name"])
			}
			m.Body = notificationLimit(title+"\n"+notificationString(item["body"]), 1200)
			for _, field := range []string{"state", "status", "conclusion", "tag_name", "content", "description"} {
				if value := notificationString(item[field]); value != "" {
					m.Body += "\n" + field + ": " + notificationLimit(value, 300)
				}
			}
			if link := notificationURL(item["html_url"]); link != "" {
				m.URL = link
			}
			if m.Body != "" {
				break
			}
		}
		m.Body = strings.TrimSpace(notificationString(data["action"]) + "\n" + m.Body)
		if m.Body == "" {
			m.Body = notificationString(data["description"])
		}
		if state := notificationString(data["state"]); state != "" {
			m.Body += "\n" + state
		}
	}
	var plain strings.Builder
	plain.WriteString(m.Body)
	for _, commit := range m.Commits {
		fmt.Fprintf(&plain, "\n%s  %s — %s", commit.SHA, commit.Message, commit.Author)
		if commit.Files != "" {
			plain.WriteString("\n" + commit.Files)
		}
	}
	if m.More > 0 {
		fmt.Fprintf(&plain, "\n+%d %s", m.More, notificationLabel(m.Chinese, "more commits", "条提交"))
	}
	var err error
	m.Title, err = renderNotificationTemplate(settings.TitleTemplate, t.EventType, m.Title, []byte(t.PayloadContent))
	if err != nil {
		return m, err
	}
	m.Title = notificationLimit(m.Title, 180)
	m.Body, err = renderNotificationTemplate(settings.MessageTemplate, t.EventType, strings.TrimSpace(plain.String()), []byte(t.PayloadContent))
	m.CustomBody = strings.TrimSpace(settings.MessageTemplate) != ""
	m.Body = notificationLimit(m.Body, 2800)
	if m.Body == "" {
		m.Body = m.Title
	}
	return m, err
}

// A first push has no comparable base. Some legacy payloads use the site root as compare_url.
func notificationPushURL(data map[string]any, repositoryURL string) string {
	after := notificationString(data["after"])
	before := notificationString(data["before"])
	isObjectID := func(value string) bool {
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
	if isObjectID(after) && !isObjectID(before) && repositoryURL != "" {
		return strings.TrimRight(repositoryURL, "/") + "/commit/" + after
	}
	if link := notificationURL(data["compare_url"]); link != "" {
		u, _ := url.Parse(link)
		// Never send a button to the site root or an all-zero compare endpoint.
		if strings.Contains(u.Path, "/compare/") && !strings.Contains(u.Path, strings.Repeat("0", 40)) {
			return link
		}
	}
	if isObjectID(after) && repositoryURL != "" {
		return strings.TrimRight(repositoryURL, "/") + "/commit/" + after
	}
	if commits, ok := data["commits"].([]any); ok && len(commits) == 1 {
		if link := notificationURL(notificationMap(commits[0])["url"]); link != "" {
			return link
		}
	}
	return repositoryURL
}

func notificationFormattedBody(m notificationMessage, format string, budget int) string {
	escape := notificationMarkdown
	switch format {
	case "html":
		escape = html.EscapeString
	case "slack":
		escape = SlackTextFormatter
	}
	if m.CustomBody || len(m.Commits) == 0 {
		body := m.Body
		for len(escape(body)) > budget && len(body) > 0 {
			body = string([]rune(body)[:len([]rune(body))/2])
		}
		return escape(body)
	}
	var body strings.Builder
	shown := 0
	for _, commit := range m.Commits {
		sha := escape(commit.SHA)
		link := strings.NewReplacer("(", "%28", ")", "%29").Replace(commit.URL)
		if link != "" {
			switch format {
			case "html":
				sha = "<a href=\"" + html.EscapeString(link) + "\">" + sha + "</a>"
			case "slack":
				sha = SlackLinkFormatter(link, commit.SHA)
			default:
				sha = "[" + sha + "](" + link + ")"
			}
		}
		line := sha + "  " + escape(commit.Message)
		if commit.Author != "" {
			line += " — " + escape(commit.Author)
		}
		if commit.Files != "" {
			line += "\n" + escape(commit.Files)
		}
		if body.Len()+len(line)+100 > budget {
			break
		}
		body.WriteString(line + "\n\n")
		shown++
	}
	if m.Total > shown {
		fmt.Fprintf(&body, "+%d %s", m.Total-shown, notificationLabel(m.Chinese, "more commits — open changes for the full list", "条提交，点击查看完整变更"))
	}
	return strings.TrimSpace(body.String())
}

func notificationRichRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	settings, err := GetNotificationSettings(w.Meta)
	if err != nil {
		return nil, nil, err
	}
	if w.Type == webhook_module.FEISHU && settings.MessageTemplate == "" && strings.TrimSpace(w.Meta) != "" {
		settings.MessageTemplate = GetFeishuHook(w).MessageTemplate
	}
	m, err := buildNotificationMessage(settings, t)
	if err != nil {
		return nil, nil, err
	}
	label := notificationLabel(m.Chinese, "Open changes", "查看变更")
	plain := notificationLimit(m.Title+" · "+m.Repository, 240)
	meta := map[string]any{}
	if w.Meta != "" {
		_ = json.Unmarshal([]byte(w.Meta), &meta)
	}
	color := settings.Color
	if color == "" {
		color = "#3b82f6"
	}
	facts := []map[string]any{}
	for _, field := range [][2]string{{notificationLabel(m.Chinese, "Repository", "仓库"), m.Repository}, {notificationLabel(m.Chinese, "Branch / tag", "分支 / 标签"), m.Branch}, {notificationLabel(m.Chinese, "Actor", "操作人"), m.Actor}, {notificationLabel(m.Chinese, "Commits", "提交数"), strconv.Itoa(m.Total)}} {
		if t.EventType != webhook_module.HookEventPush && field[0] == notificationLabel(m.Chinese, "Commits", "提交数") {
			continue
		}
		if field[1] != "" {
			facts = append(facts, map[string]any{"title": field[0], "value": notificationLimit(field[1], 160)})
		}
	}
	var payload any
	switch w.Type {
	case webhook_module.FEISHU:
		fields := []any{}
		for _, fact := range facts {
			fields = append(fields, map[string]any{"is_short": true, "text": map[string]any{"tag": "lark_md", "content": "**" + fact["title"].(string) + "**\n" + notificationMarkdown(fact["value"].(string))}})
		}
		elements := []any{map[string]any{"tag": "div", "fields": fields}, map[string]any{"tag": "hr"}, map[string]any{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": notificationFormattedBody(m, "markdown", 6000)}}}
		if m.URL != "" {
			elements = append(elements, map[string]any{"tag": "action", "actions": []any{map[string]any{"tag": "button", "text": map[string]any{"tag": "plain_text", "content": label}, "type": "primary", "url": m.URL}}})
		}
		elements = append(elements, map[string]any{"tag": "note", "elements": []any{map[string]any{"tag": "plain_text", "content": m.Footer}}})
		value := map[string]any{"msg_type": "interactive", "card": map[string]any{"config": map[string]any{"wide_screen_mode": true}, "header": map[string]any{"template": "blue", "title": map[string]any{"tag": "plain_text", "content": plain}}, "elements": elements}}
		if w.Secret != "" {
			now := time.Now().Unix()
			value["timestamp"], value["sign"] = now, feishuGenSign(w.Secret, now)
		}
		payload = value
	case webhook_module.DISCORD:
		fields := []any{}
		for _, fact := range facts {
			fields = append(fields, map[string]any{"name": fact["title"], "value": notificationMarkdown(fact["value"].(string)), "inline": true})
		}
		colorValue, _ := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32)
		payload = map[string]any{"username": meta["username"], "avatar_url": meta["icon_url"], "allowed_mentions": map[string]any{"parse": []string{}}, "embeds": []any{map[string]any{"title": plain, "description": notificationFormattedBody(m, "markdown", 3900), "url": m.URL, "color": colorValue, "fields": fields, "footer": map[string]any{"text": m.Footer}}}}
	case webhook_module.TELEGRAM:
		var body strings.Builder
		body.WriteString("<b>" + html.EscapeString(plain) + "</b>\n")
		for _, fact := range facts {
			body.WriteString("<b>" + html.EscapeString(fact["title"].(string)) + ":</b> " + html.EscapeString(fact["value"].(string)) + "\n")
		}
		body.WriteString("\n" + notificationFormattedBody(m, "html", 2300) + "\n\n<i>" + html.EscapeString(m.Footer) + "</i>")
		value := map[string]any{"text": body.String(), "parse_mode": "HTML", "disable_web_page_preview": true}
		if m.URL != "" {
			value["reply_markup"] = map[string]any{"inline_keyboard": []any{[]any{map[string]any{"text": label, "url": m.URL}}}}
		}
		payload = value
	case webhook_module.WECHATWORK, webhook_module.DINGTALK:
		text := "### " + notificationMarkdown(plain) + "\n"
		for _, fact := range facts {
			text += "> **" + fact["title"].(string) + "**: " + notificationMarkdown(fact["value"].(string)) + "\n"
		}
		ending := "\n\n" + notificationMarkdown(m.Footer)
		if m.URL != "" {
			ending += "\n[" + label + "](" + m.URL + ")"
		}
		text = notificationBytes(text, 1200)
		text += "\n" + notificationFormattedBody(m, "markdown", 4000-len(text)-len(ending)-1) + ending
		if w.Type == webhook_module.WECHATWORK {
			payload = map[string]any{"msgtype": "markdown", "markdown": map[string]any{"content": text}}
		} else {
			payload = map[string]any{"msgtype": "markdown", "markdown": map[string]any{"title": plain, "text": text}, "at": map[string]any{"isAtAll": false}}
		}
	case webhook_module.SLACK:
		text := notificationFormattedBody(m, "slack", 2900)
		fields := []any{}
		for _, fact := range facts {
			fields = append(fields, map[string]any{"type": "mrkdwn", "text": "*" + fact["title"].(string) + "*\n" + SlackTextFormatter(fact["value"].(string))})
		}
		blocks := []any{map[string]any{"type": "header", "text": map[string]any{"type": "plain_text", "text": notificationLimit(plain, 140)}}, map[string]any{"type": "section", "fields": fields}, map[string]any{"type": "section", "text": map[string]any{"type": "mrkdwn", "text": text}}, map[string]any{"type": "context", "elements": []any{map[string]any{"type": "plain_text", "text": m.Footer}}}}
		if m.URL != "" {
			blocks = append(blocks, map[string]any{"type": "actions", "elements": []any{map[string]any{"type": "button", "text": map[string]any{"type": "plain_text", "text": label}, "url": m.URL}}})
		}
		payload = map[string]any{"text": plain, "channel": meta["channel"], "username": meta["username"], "icon_url": meta["icon_url"], "unfurl_links": false, "unfurl_media": false, "attachments": []any{map[string]any{"color": color, "blocks": blocks}}}
	case webhook_module.MSTEAMS:
		payload = map[string]any{"type": "message", "attachments": []any{map[string]any{"contentType": "application/vnd.microsoft.card.adaptive", "content": map[string]any{"type": "AdaptiveCard", "version": "1.4", "body": []any{map[string]any{"type": "TextBlock", "text": plain, "weight": "Bolder", "size": "Medium", "wrap": true}, map[string]any{"type": "FactSet", "facts": facts}, map[string]any{"type": "TextBlock", "text": notificationMarkdown(m.Body), "wrap": true}, map[string]any{"type": "TextBlock", "text": m.Footer, "isSubtle": true, "wrap": true}}, "actions": []any{map[string]any{"type": "Action.OpenUrl", "title": label, "url": m.URL}}}}}}
	case webhook_module.MATRIX:
		var factText strings.Builder
		for _, fact := range facts {
			factText.WriteString(fact["title"].(string) + ": " + fact["value"].(string) + "\n")
		}
		payload = map[string]any{"msgtype": "m.notice", "body": plain + "\n" + factText.String() + m.Body, "format": "org.matrix.custom.html", "formatted_body": "<h3>" + html.EscapeString(plain) + "</h3><p>" + strings.ReplaceAll(html.EscapeString(factText.String()), "\n", "<br>") + "</p><p>" + strings.ReplaceAll(notificationFormattedBody(m, "html", 6000), "\n", "<br>") + "</p><p><a href=\"" + html.EscapeString(m.URL) + "\">" + label + "</a></p><p>" + html.EscapeString(m.Footer) + "</p>"}
	default:
		return nil, nil, errors.New("unsupported notification channel")
	}
	if value, ok := payload.(map[string]any); ok {
		for _, key := range []string{"username", "avatar_url", "icon_url", "channel"} {
			if value[key] == nil || value[key] == "" {
				delete(value, key)
			}
		}
	}
	req, body, err := prepareJSONRequest(payload, w, t, w.Type != webhook_module.FEISHU)
	if err == nil && w.Type == webhook_module.MATRIX {
		txn, txnErr := getMatrixTxnID(body)
		if txnErr != nil {
			return nil, nil, txnErr
		}
		req.Method = http.MethodPut
		req.URL, err = url.Parse(w.URL + "/" + txn)
	}
	if req != nil {
		req = req.WithContext(ctx)
	}
	return req, body, err
}
