// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

var notificationAliasNames = []string{"event", "text", "repository", "repositoryUrl", "branch", "actor", "actorName", "actorDisplay", "commitMessage", "commitMessages", "commitAuthor", "trackId", "commitUrl", "compareUrl", "timestamp", "action", "title", "body", "footer", "color", "page", "pages"}

func notificationAliases(event webhook_module.HookEventType, text string, data map[string]any) map[string]string {
	values := make(map[string]string, len(notificationAliasNames))
	for _, name := range notificationAliasNames {
		values[name] = ""
	}
	values["event"], values["text"] = string(event), text
	values["repository"] = notificationString(notificationMap(data["repository"])["full_name"])
	values["repositoryUrl"] = notificationRepositoryURL(data)
	values["branch"] = strings.TrimPrefix(strings.TrimPrefix(notificationString(data["ref"]), "refs/heads/"), "refs/tags/")
	if values["branch"] == "" {
		values["branch"] = notificationString(notificationMap(data["release"])["tag_name"])
	}
	values["actor"], values["actorDisplay"] = notificationActor(data), notificationActorDisplay(data)
	for _, key := range []string{"sender", "pusher", "user"} {
		actor := notificationMap(data[key])
		if notificationActor(map[string]any{"sender": actor}) == values["actor"] {
			values["actorName"] = notificationString(actor["full_name"])
			if values["actorName"] == "" {
				values["actorName"] = notificationString(actor["name"])
			}
			break
		}
	}
	head := notificationMap(data["head_commit"])
	commits, _ := data["commits"].([]any)
	messages := make([]string, 0, len(commits))
	for _, commit := range commits {
		row := notificationMap(commit)
		message, _ := row["message"].(string)
		messages = append(messages, message)
		if head == nil {
			// Payload commit order is chronological; the final row is the pushed head.
			values["commitMessage"] = message
			values["commitAuthor"] = notificationString(notificationMap(row["author"])["name"])
			values["timestamp"] = notificationString(row["timestamp"])
		}
	}
	if head != nil {
		values["commitMessage"], _ = head["message"].(string)
		values["commitAuthor"] = notificationString(notificationMap(head["author"])["name"])
		values["timestamp"] = notificationString(head["timestamp"])
	}
	values["commitMessages"] = strings.Join(messages, "\n\n")
	if values["commitMessages"] == "" {
		values["commitMessages"] = values["commitMessage"]
	}
	values["trackId"], values["commitUrl"] = notificationTrack(data, &webhook_model.HookTask{})
	values["compareUrl"] = notificationPublicRepositoryLink(data, notificationPushURL(data, values["repositoryUrl"]))
	values["action"] = notificationString(data["action"])
	for _, key := range []string{"workflow_run", "workflow_job"} {
		workflow := notificationMap(data[key])
		if values["branch"] == "" {
			values["branch"] = notificationString(workflow["head_branch"])
		}
	}
	return values
}

func notificationAliasFunctions(values map[string]string) template.FuncMap {
	funcs := template.FuncMap{"json": func(value any) (string, error) {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}}
	for _, name := range notificationAliasNames {
		funcs[name] = func() string { return values[name] }
	}
	return funcs
}

var notificationJSONPlaceholder = regexp.MustCompile(`\{\{\s*(?:json\s+)?([a-zA-Z][a-zA-Z0-9]*)\s*\}\}`)

// JSON placeholders are encoded according to their string context, never reparsed as template source.
func renderNotificationJSON(source string, values map[string]string) (map[string]any, error) {
	if len(source) > MaxNotificationTemplateSize {
		return nil, errors.New("JSON notification template exceeds 16 KiB")
	}
	var output notificationOutput
	inString, escaped := false, false
	writeLiteral := func(literal string) error {
		if strings.Contains(literal, "{{") {
			return errors.New("JSON templates support named placeholders only")
		}
		for _, char := range literal {
			if escaped {
				escaped = false
			} else if inString && char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = !inString
			}
		}
		_, err := output.WriteString(literal)
		return err
	}
	previous := 0
	for _, match := range notificationJSONPlaceholder.FindAllStringSubmatchIndex(source, -1) {
		if err := writeLiteral(source[previous:match[0]]); err != nil {
			return nil, err
		}
		name := source[match[2]:match[3]]
		value, exists := values[name]
		if !exists || escaped {
			return nil, fmt.Errorf("unknown or escaped JSON template variable: %s", name)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		if inString {
			encoded = encoded[1 : len(encoded)-1]
		}
		if _, err := output.Write(encoded); err != nil {
			return nil, err
		}
		previous = match[1]
	}
	if err := writeLiteral(source[previous:]); err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil || len(payload) == 0 {
		return nil, errors.New("notification template must render a nonempty JSON object")
	}
	return payload, nil
}

func validateNotificationJSONChannel(kind webhook_module.HookType, payload map[string]any) error {
	var required []string
	switch kind {
	case webhook_module.EMAIL:
		required = []string{"subject", "text"}
	case webhook_module.FEISHU:
		if notificationString(payload["msg_type"]) == "interactive" {
			required = []string{"card"}
		} else {
			required = []string{"msg_type", "content"}
		}
	case webhook_module.TELEGRAM:
		required = []string{"text"}
	case webhook_module.DISCORD:
		if payload["embeds"] == nil {
			required = []string{"content"}
		}
	case webhook_module.WECHATWORK, webhook_module.DINGTALK:
		required = []string{"msgtype", notificationString(payload["msgtype"])}
	case webhook_module.SLACK:
		if payload["blocks"] == nil && payload["attachments"] == nil {
			required = []string{"text"}
		}
	case webhook_module.MSTEAMS:
		required = []string{"type", "attachments"}
	case webhook_module.MATRIX:
		required = []string{"msgtype", "body"}
	default:
		return errors.New("unsupported custom notification channel")
	}
	for _, key := range required {
		if payload[key] == nil || payload[key] == "" {
			return fmt.Errorf("%s JSON template requires %s", kind, key)
		}
	}
	return nil
}

// ValidateForChannel checks the provider envelope before storing a template.
func (n NotificationSettings) ValidateForChannel(kind webhook_module.HookType) error {
	if kind == webhook_module.EMAIL && n.Style == "legacy" {
		return errors.New("email notifications require rich presentation")
	}
	if err := n.Validate(); err != nil || n.PayloadTemplate == "" {
		return err
	}
	values := notificationAliases("push", "example", map[string]any{})
	for key := range values {
		values[key] = "example"
	}
	payload, err := renderNotificationJSON(n.PayloadTemplate, values)
	if err != nil {
		return err
	}
	return validateNotificationJSONChannel(kind, payload)
}
