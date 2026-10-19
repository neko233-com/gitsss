// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"errors"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	sender_service "github.com/neko233-com/gitsss/services/mailer/sender"
)

// SendWebhookNotification reports SMTP acceptance synchronously to the hook task.
func SendWebhookNotification(to, subject, body string) error {
	if setting.MailService == nil || sender == nil || setting.MailService.Protocol == "dummy" {
		return errors.New("notification email requires an enabled real SMTP or sendmail service")
	}
	transport := sender
	if setting.MailService.Protocol != "sendmail" {
		timeout := max(setting.Webhook.DeliverTimeout, 1)
		transport = &sender_service.SMTPSender{Timeout: time.Duration(timeout) * time.Second}
	}
	return sender_service.Send(transport, sender_service.NewMessage(to, sanitizeSubject(subject), body))
}
