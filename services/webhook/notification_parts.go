// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

type notificationPart struct {
	Request *http.Request
	Body    []byte
}

var notificationPartInterval = func(kind webhook_module.HookType) time.Duration {
	switch kind {
	case webhook_module.WECHATWORK, webhook_module.DINGTALK:
		return 3100 * time.Millisecond
	case webhook_module.SLACK:
		return 1100 * time.Millisecond
	case webhook_module.FEISHU:
		return 300 * time.Millisecond
	default:
		return 100 * time.Millisecond
	}
}

// Check the actual encoded envelope, including metadata and template escaping.
func notificationPayloadFits(kind webhook_module.HookType, body []byte) bool {
	limit := 28000
	switch kind {
	case webhook_module.FEISHU:
		limit = 19000
	case webhook_module.EMAIL, webhook_module.MATRIX:
		limit = 60000
	case webhook_module.MSTEAMS:
		limit = 26000
	case webhook_module.DINGTALK:
		limit = 19000
	}
	if len(body) > limit {
		return false
	}
	var value map[string]any
	if json.Unmarshal(body, &value) != nil {
		return false
	}
	count := func(text any) int { s, _ := text.(string); return utf8.RuneCountInString(s) }
	switch kind {
	case webhook_module.TELEGRAM:
		return count(value["text"]) <= 4000
	case webhook_module.WECHATWORK:
		for _, name := range []string{"markdown", "text"} {
			if text := notificationMap(value[name]); text != nil {
				s, _ := text["content"].(string)
				if len(s) > 4000 {
					return false
				}
			}
		}
	case webhook_module.DISCORD:
		if count(value["content"]) > 1900 {
			return false
		}
		embeds, _ := value["embeds"].([]any)
		total := 0
		for _, raw := range embeds {
			embed := notificationMap(raw)
			if count(embed["description"]) > 4000 || count(embed["title"]) > 256 {
				return false
			}
			total += count(embed["description"]) + count(embed["title"]) + count(notificationMap(embed["footer"])["text"])
			fields, _ := embed["fields"].([]any)
			for _, raw := range fields {
				field := notificationMap(raw)
				if count(field["value"]) > 1024 {
					return false
				}
				total += count(field["name"]) + count(field["value"])
			}
		}
		return total <= 5800
	case webhook_module.SLACK:
		var walk func(any) bool
		walk = func(raw any) bool {
			switch v := raw.(type) {
			case map[string]any:
				if text, ok := v["text"].(string); ok && utf8.RuneCountInString(text) > 2900 {
					return false
				}
				for _, child := range v {
					if !walk(child) {
						return false
					}
				}
			case []any:
				for _, child := range v {
					if !walk(child) {
						return false
					}
				}
			}
			return true
		}
		return walk(value)
	}
	return true
}

func notificationRichRequests(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) ([]notificationPart, error) {
	settings, err := GetNotificationSettings(w.Meta)
	if err != nil {
		return nil, err
	}
	if w.Type == webhook_module.FEISHU && settings.MessageTemplate == "" {
		settings.MessageTemplate = GetFeishuHook(w).MessageTemplate
	}
	m, err := buildNotificationMessage(settings, t)
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	if err = json.Unmarshal([]byte(t.PayloadContent), &data); err != nil {
		return nil, err
	}
	if t.EventType == webhook_module.HookEventPush {
		m.ActionStatus = notificationPushActions(ctx, data, m.Chinese)
	}
	render := func(message notificationMessage) (notificationPart, error) {
		req, body, err := notificationRichRequestPart(ctx, w, t, &message)
		return notificationPart{req, body}, err
	}
	whole, err := render(m)
	if err == nil && notificationPayloadFits(w.Type, whole.Body) {
		return []notificationPart{whole}, nil
	}
	if err != nil && !notificationSizeError(err) {
		return nil, err
	}
	source := m.Body
	if w.Type == webhook_module.FEISHU && !m.CustomBody && len(m.Commits) > 0 {
		messages := make([]string, 0, len(m.Commits))
		for _, commit := range m.Commits {
			messages = append(messages, commit.Message)
		}
		source = strings.Join(messages, "\n\n")
		if m.More > 0 {
			source += "\n\n" + notificationLabel(m.Chinese, "The source payload omitted further commits; open changes for the full list.", "源事件未附带其余提交，请查看变更阅读完整列表。")
		}
	}
	if settings.PayloadTemplate != "" {
		aliases := notificationAliases(t.EventType, m.Body, data)
		if strings.Contains(settings.PayloadTemplate, "commitMessages") {
			source = aliases["commitMessages"]
		} else if strings.Contains(settings.PayloadTemplate, "commitMessage") {
			source = aliases["commitMessage"]
		}
	}
	if len(source) > 1024*1024 || !utf8.ValidString(source) {
		return nil, errors.New("notification body exceeds the 1 MiB safety limit or contains invalid UTF-8; no parts sent")
	}
	runes := []rune(source)
	chunks := []string{}
	for offset := 0; offset < len(runes); {
		if len(chunks) >= 128 {
			return nil, errors.New("notification requires more than 128 parts; no parts sent")
		}
		low, high := 0, len(runes)-offset
		for low < high {
			mid := (low + high + 1) / 2
			page := m
			page.Body = string(runes[offset : offset+mid])
			page.Commits = nil
			page.CustomBody = true
			page.Page, page.Pages = 128, 128
			candidate, err := render(page)
			if err != nil && !notificationSizeError(err) {
				return nil, err
			}
			if err == nil && notificationPayloadFits(w.Type, candidate.Body) {
				low = mid
			} else {
				high = mid - 1
			}
		}
		if low == 0 {
			return nil, errors.New("notification metadata or fixed template exceeds the provider limit; no parts sent")
		}
		chunks = append(chunks, string(runes[offset:offset+low]))
		offset += low
	}
	if len(chunks) == 0 {
		return nil, errors.New("notification metadata exceeds the provider limit; no parts sent")
	}
	parts := make([]notificationPart, 0, len(chunks))
	for index, chunk := range chunks {
		page := m
		page.Body = chunk
		page.Commits = nil
		page.CustomBody = true
		page.Page, page.Pages = index+1, len(chunks)
		part, err := render(page)
		if err != nil {
			return nil, err
		}
		if !notificationPayloadFits(w.Type, part.Body) {
			return nil, fmt.Errorf("notification part %d exceeds the provider limit; no parts sent", index+1)
		}
		part.Request.Header.Set("X-GitSSS-Part", strconv.Itoa(index+1))
		part.Request.Header.Set("X-GitSSS-Parts", strconv.Itoa(len(chunks)))
		parts = append(parts, part)
	}
	return parts, nil
}

func notificationSizeError(err error) bool {
	return strings.Contains(err.Error(), "exceeds 64 KiB") || strings.HasPrefix(err.Error(), "getMatrixTxnID: payload size ")
}
