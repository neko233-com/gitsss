// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	sender_service "github.com/neko233-com/gitsss/services/mailer/sender"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookMailSMTPAcceptance(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	config := &setting.Mailer{Protocol: "smtp", SMTPAddr: host, SMTPPort: port, FromEmail: "git@example.invalid", FromName: "GitSSS"}
	defer test.MockVariableValue(&setting.MailService, config)()
	defer test.MockVariableValue(&sender, sender_service.Sender(&sender_service.SMTPSender{}))()
	defer test.MockVariableValue(&setting.Webhook.DeliverTimeout, 2)()
	delivered := make(chan string, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		_, _ = fmt.Fprint(conn, "220 fixture SMTP\r\n")
		reader := bufio.NewReader(conn)
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "DATA"):
				_, _ = fmt.Fprint(conn, "354 send message\r\n")
				var body strings.Builder
				for {
					row, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if row == ".\r\n" {
						break
					}
					body.WriteString(row)
				}
				delivered <- body.String()
				_, _ = fmt.Fprint(conn, "250 accepted\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "250 fixture\r\n")
			}
		}
	}()
	require.NoError(t, SendWebhookNotification("release@example.invalid", "Release v1.2.3", "<html><body>release fixture</body></html>"))
	select {
	case body := <-delivered:
		assert.Contains(t, body, "release@example.invalid")
		assert.Contains(t, body, "Release v1.2.3")
		assert.Contains(t, body, "release fixture")
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP did not receive notification")
	}
	setting.MailService = nil
	require.Error(t, SendWebhookNotification("release@example.invalid", "fixture", "fixture"))
}
