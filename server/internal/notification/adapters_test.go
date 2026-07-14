// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

type recordingSMTP struct {
	from    string
	message string
}

func (transport *recordingSMTP) Send(_ string, _ smtp.Auth, from string, _ []string, message []byte) error {
	transport.from = from
	transport.message = string(message)
	return nil
}

func TestFeishuAdapterSignsTextPayload(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	now := time.Unix(1599360473, 0)
	adapter := webhookAdapter{channel: ChannelFeishu, endpoint: server.URL, secret: "demo", client: server.Client(), now: func() time.Time { return now }}
	if err := adapter.Deliver(context.Background(), Message{Subject: "构建失败", Body: "请检查"}); err != nil {
		t.Fatal(err)
	}
	if payload["timestamp"] != "1599360473" || payload["sign"] != feishuSignature("1599360473", "demo") {
		t.Fatalf("signature payload = %#v", payload)
	}
}

func TestFactoryRestrictsWebhookHosts(t *testing.T) {
	factory := NewFactory()
	_, err := factory.Build(Configuration{Channel: ChannelWeCom, WebhookURL: "https://example.com/cgi-bin/webhook/send?key=secret"})
	if err == nil || !strings.Contains(err.Error(), "official") {
		t.Fatalf("error = %v, want official host rejection", err)
	}
}

func TestFactoryAcceptsOfficialWebhookShapes(t *testing.T) {
	factory := NewFactory()
	if _, err := factory.Build(Configuration{Channel: ChannelFeishu, WebhookURL: "https://open.feishu.cn/open-apis/bot/v2/hook/id"}); err != nil {
		t.Fatal(err)
	}
	if _, err := factory.Build(Configuration{Channel: ChannelWeCom, WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=id"}); err != nil {
		t.Fatal(err)
	}
}

func TestEmailAdapterUsesParsedEnvelopeAddressAndEncodedSubject(t *testing.T) {
	transport := &recordingSMTP{}
	adapter, err := newEmailAdapter(Configuration{
		Channel: ChannelEmail, SMTPHost: "smtp.example.com", SMTPPort: 587,
		From: "gitsss 团队 <gitsss@example.com>", Recipients: []string{"ops@example.com"},
	}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Deliver(context.Background(), Message{Subject: "构建失败", Body: "请检查"}); err != nil {
		t.Fatal(err)
	}
	if transport.from != "gitsss@example.com" || !strings.Contains(transport.message, "Subject: =?UTF-8?q?") {
		t.Fatalf("SMTP envelope = %q, message = %q", transport.from, transport.message)
	}
}
