// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"errors"
	"fmt"
	"html"
	"net/mail"
	"strings"
	"time"

	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/services/mailer"
)

func notificationEmailPayload(m notificationMessage) map[string]any {
	fields := [][2]string{
		{notificationLabel(m.Chinese, "Repository", "仓库"), m.Repository},
		{notificationLabel(m.Chinese, "Branch / tag", "分支 / 标签"), m.Branch},
		{notificationLabel(m.Chinese, "Actor", "操作人"), m.Actor},
		{"trackId", m.TrackID},
		{"CI/CD · Actions", m.ActionStatus},
	}
	if m.Timestamp != "" {
		fields = append(fields, [2]string{notificationLabel(m.Chinese, "Time", "时间"), m.Timestamp})
	}
	var text strings.Builder
	for _, field := range fields {
		if field[1] != "" {
			fmt.Fprintf(&text, "%s: %s\n", field[0], field[1])
		}
	}
	text.WriteString("\n" + m.Body + "\n\n" + m.URL + "\n\n" + m.Footer)
	return map[string]any{"subject": m.Title + " · " + m.Repository, "text": text.String(), "heading": m.Title, "message": m.Body, "repository": m.Repository, "branch": m.Branch, "actor": m.Actor, "trackId": m.TrackID, "trackUrl": m.TrackURL, "url": m.URL, "timestamp": m.Timestamp, "actions": m.ActionStatus, "footer": m.Footer, "chinese": m.Chinese}
}

func notificationEmailHTML(payload map[string]any, subject, text string) string {
	chinese, _ := payload["chinese"].(bool)
	label := func(en, zh string) string { return notificationLabel(chinese, en, zh) }
	escape := html.EscapeString
	heading := notificationString(payload["heading"])
	if heading == "" {
		heading = subject
	}
	message, ok := payload["message"].(string)
	if !ok {
		message = text
	}
	var facts strings.Builder
	fields := [][2]string{{label("Repository", "仓库"), notificationString(payload["repository"])}, {label("Branch / tag", "分支 / 标签"), notificationString(payload["branch"])}, {label("Actor", "操作人"), notificationString(payload["actor"])}, {"trackId", notificationString(payload["trackId"])}, {"CI/CD · Actions", notificationString(payload["actions"])}}
	stamp := notificationString(payload["timestamp"])
	if parsed, err := time.Parse(time.RFC3339, stamp); err == nil {
		stamp = parsed.In(setting.DefaultUILocation).Format("2006-01-02 15:04:05 Mon MST")
	}
	if stamp != "" {
		fields = append(fields, [2]string{label("Time", "时间"), stamp})
	}
	count := 0
	for _, field := range fields {
		if field[1] == "" {
			continue
		}
		if count%2 == 0 {
			facts.WriteString("<tr>")
		}
		value := escape(field[1])
		if field[0] == "trackId" {
			if link := notificationURL(payload["trackUrl"]); link != "" {
				value = "<a style=\"color:#0969da;text-decoration:none\" href=\"" + escape(link) + "\">" + value + "</a>"
			}
		}
		fmt.Fprintf(&facts, "<td width=\"50%%\" valign=\"top\" style=\"padding:10px 16px 10px 0;word-break:break-word\"><div style=\"font-size:12px;color:#59636e;margin-bottom:4px\">%s</div><div style=\"font-size:14px;color:#1f2328;font-weight:600\">%s</div></td>", escape(field[0]), value)
		count++
		if count%2 == 0 {
			facts.WriteString("</tr>")
		}
	}
	if count%2 != 0 {
		facts.WriteString("<td></td></tr>")
	}
	button := ""
	if link := notificationURL(payload["url"]); link != "" {
		button = "<table role=\"presentation\" cellspacing=\"0\" cellpadding=\"0\" style=\"margin-top:24px\"><tr><td bgcolor=\"#0969da\" style=\"border-radius:7px\"><a href=\"" + escape(link) + "\" style=\"display:inline-block;padding:12px 22px;color:#ffffff;font-size:14px;font-weight:600;text-decoration:none\">" + label("View changes →", "查看变更 →") + "</a></td></tr></table>"
	}
	footer := notificationString(payload["footer"])
	if footer == "" {
		footer = "GitSSS · " + label("Engineering collaboration", "研发协作")
	}
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;padding:0;background:#f6f8fa;color:#1f2328;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',Arial,sans-serif"><table role="presentation" width="100%" cellspacing="0" cellpadding="0" bgcolor="#f6f8fa"><tr><td align="center" style="padding:32px 16px"><table role="presentation" width="720" cellspacing="0" cellpadding="0" style="width:100%;max-width:720px"><tr><td style="padding:0 4px 18px;font-size:19px;font-weight:700;color:#0969da">● GitSSS <span style="font-size:12px;font-weight:400;color:#59636e"> / ` + escape(label("Engineering updates", "研发动态")) + `</span></td></tr><tr><td bgcolor="#ffffff" style="border:1px solid #d1d9e0;border-top:4px solid #0969da;border-radius:10px;padding:28px 30px"><h1 style="margin:0 0 20px;font-size:23px;line-height:1.4;color:#1f2328">` + escape(heading) + `</h1><table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border-top:1px solid #e5e9ef;border-bottom:1px solid #e5e9ef;margin-bottom:24px">` + facts.String() + `</table><h2 style="margin:0 0 12px;font-size:13px;color:#59636e;font-weight:600">` + escape(label("Details", "变更详情")) + `</h2><div style="font-size:15px;line-height:1.8;color:#1f2328;white-space:pre-wrap;overflow-wrap:anywhere;word-break:break-word">` + escape(message) + `</div>` + button + `</td></tr><tr><td style="padding:18px 4px;font-size:12px;line-height:1.6;color:#59636e">` + escape(footer) + `<br>` + escape(label("Sent by your GitSSS notification policy. Keep this message for traceability.", "由 GitSSS 通知策略发送，便于追踪本次变更。")) + `</td></tr></table></td></tr></table></body></html>`
}

func notificationEmailRecipients(source string) ([]string, error) {
	if len(source) > 4096 {
		return nil, errors.New("email recipient list exceeds 4096 bytes")
	}
	addresses, err := mail.ParseAddressList(source)
	if err != nil || len(addresses) == 0 || len(addresses) > 20 || strings.ContainsAny(source, "\r\n") {
		return nil, errors.New("enter 1–20 valid comma-separated email recipients without line breaks")
	}
	result := make([]string, 0, len(addresses))
	seen := map[string]bool{}
	for _, address := range addresses {
		key := strings.ToLower(address.Address)
		if !strings.Contains(key, "@") {
			return nil, errors.New("email recipients must be complete addresses")
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, address.Address)
		}
	}
	return result, nil
}

func deliverNotificationEmail(w *webhook_model.Webhook, body []byte) error {
	meta := map[string]any{}
	if err := json.Unmarshal([]byte(w.Meta), &meta); err != nil {
		return err
	}
	recipients, err := notificationEmailRecipients(notificationString(meta["recipients"]))
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	subject, text := notificationString(payload["subject"]), notificationString(payload["text"])
	if subject == "" || text == "" || strings.ContainsAny(subject, "\r\n") {
		return errors.New("email templates require a single-line subject and nonempty text")
	}
	content := notificationEmailHTML(payload, subject, text)
	// One envelope hides other recipients; a partial SMTP failure is surfaced, never silently retried.
	for _, recipient := range recipients {
		if err := mailer.SendWebhookNotification(recipient, subject, content); err != nil {
			return err
		}
	}
	return nil
}
