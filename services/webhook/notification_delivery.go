// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

// NotificationConfigMeta merges API edits without dropping channel credentials or routing policy.
func NotificationConfigMeta(kind webhook_module.HookType, existing string, config map[string]string) (string, error) {
	fields := map[string]any{}
	if existing != "" {
		if err := json.Unmarshal([]byte(existing), &fields); err != nil {
			return "", err
		}
	}
	keys := []string{}
	switch kind {
	case webhook_module.EMAIL:
		keys = []string{"recipients"}
	case webhook_module.SLACK:
		keys = []string{"channel", "username", "icon_url", "color"}
	case webhook_module.DISCORD:
		keys = []string{"username", "icon_url"}
	case webhook_module.TELEGRAM:
		keys = []string{"bot_token", "chat_id", "thread_id"}
	case webhook_module.MATRIX:
		keys = []string{"homeserver_url", "room_id"}
	case webhook_module.FEISHU:
		keys = []string{"message_template"}
	}
	for _, key := range keys {
		if value, ok := config[key]; ok {
			fields[key] = strings.TrimSpace(value)
		}
	}
	if kind == webhook_module.EMAIL {
		if _, err := notificationEmailRecipients(notificationString(fields["recipients"])); err != nil {
			return "", err
		}
	}
	if kind == webhook_module.SLACK && !IsValidSlackChannel(notificationString(fields["channel"])) {
		return "", errors.New("invalid Slack channel")
	}
	if source, ok := config["notification"]; ok {
		settings, _ := GetNotificationSettings("")
		if err := json.Unmarshal([]byte(source), &settings); err != nil {
			return "", errors.New("invalid notification configuration JSON")
		}
		if err := settings.ValidateForChannel(kind); err != nil {
			return "", err
		}
		fields["notification"] = settings
	}
	data, err := json.Marshal(fields)
	return string(data), err
}

func notificationSafeURL(kind webhook_module.HookType, raw string) string {
	if kind == webhook_module.EMAIL {
		return "[configured mail transport]"
	}
	if !NotificationWebhookSupported(kind) {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "[notification endpoint]"
	}
	return u.Scheme + "://" + u.Host + "/[redacted]"
}

// Providers can reject a message while returning HTTP 200.
func notificationResponseError(kind webhook_module.HookType, body []byte) error {
	fields := map[string]any{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil // Slack uses a plain-text "ok" and Discord commonly returns no body.
	}
	codeKey := ""
	switch kind {
	case webhook_module.FEISHU:
		codeKey = "code"
		if _, exists := fields[codeKey]; !exists {
			codeKey = "StatusCode"
		}
	case webhook_module.WECHATWORK, webhook_module.DINGTALK:
		codeKey = "errcode"
	case webhook_module.TELEGRAM, webhook_module.SLACK:
		if ok, exists := fields["ok"].(bool); exists && !ok {
			return fmt.Errorf("%s rejected the notification; inspect the delivery response", kind)
		}
	case webhook_module.MATRIX:
		if fields["errcode"] != nil {
			return errors.New("Matrix rejected the notification; inspect the delivery response")
		}
	}
	if code, exists := fields[codeKey].(float64); exists && code != 0 {
		return fmt.Errorf("%s rejected the notification (code %.0f)", kind, code)
	}
	return nil
}
