// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SMTPTransport interface {
	Send(context.Context, string, string, int, smtp.Auth, string, []string, []byte) error
}

type HTTPSMTPFactory struct {
	Client *http.Client
	SMTP   SMTPTransport
	Now    func() time.Time
}

func NewFactory() HTTPSMTPFactory {
	return HTTPSMTPFactory{
		Client: &http.Client{Timeout: 10 * time.Second},
		SMTP:   smtpTransport{},
		Now:    time.Now,
	}
}

func (f HTTPSMTPFactory) Build(configuration Configuration) (Adapter, error) {
	switch configuration.Channel {
	case ChannelFeishu:
		if err := validateWebhook(configuration.WebhookURL, "open.feishu.cn", "/open-apis/bot/v2/hook/"); err != nil {
			return nil, err
		}
		return webhookAdapter{channel: ChannelFeishu, endpoint: configuration.WebhookURL, secret: configuration.Secret, client: f.Client, now: f.Now}, nil
	case ChannelWeCom:
		if err := validateWebhook(configuration.WebhookURL, "qyapi.weixin.qq.com", "/cgi-bin/webhook/send"); err != nil {
			return nil, err
		}
		parsed, _ := url.Parse(configuration.WebhookURL)
		if parsed.EscapedPath() != "/cgi-bin/webhook/send" {
			return nil, errors.New("invalid WeCom webhook path")
		}
		if strings.TrimSpace(parsed.Query().Get("key")) == "" {
			return nil, errors.New("WeCom webhook key is required")
		}
		return webhookAdapter{channel: ChannelWeCom, endpoint: configuration.WebhookURL, client: f.Client, now: f.Now}, nil
	case ChannelEmail:
		return newEmailAdapter(configuration, f.SMTP)
	default:
		return nil, ErrInvalidChannel
	}
}

type webhookAdapter struct {
	channel  Channel
	endpoint string
	secret   string
	client   *http.Client
	now      func() time.Time
}

func (a webhookAdapter) Deliver(ctx context.Context, message Message) error {
	payload := map[string]any{}
	if a.channel == ChannelFeishu {
		payload = map[string]any{"msg_type": "text", "content": map[string]string{"text": message.Subject + "\n" + message.Body}}
		if a.secret != "" {
			timestamp := strconv.FormatInt(a.now().Unix(), 10)
			payload["timestamp"] = timestamp
			payload["sign"] = feishuSignature(timestamp, a.secret)
		}
	} else {
		payload = map[string]any{"msgtype": "markdown", "markdown": map[string]string{"content": "**" + message.Subject + "**\n" + message.Body}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("invalid webhook request")
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := a.client.Do(request)
	if err != nil {
		return errors.New("webhook request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Code    int `json:"code"`
		ErrCode int `json:"errcode"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 64<<10)).Decode(&result); err != nil {
		return errors.New("webhook returned an invalid response")
	}
	if result.Code != 0 || result.ErrCode != 0 {
		return errors.New("webhook rejected the message")
	}
	return nil
}

func feishuSignature(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func validateWebhook(rawURL, expectedHost, expectedPath string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != expectedHost || parsed.Port() != "" || parsed.User != nil || !strings.HasPrefix(parsed.EscapedPath(), expectedPath) {
		return fmt.Errorf("webhook must use the official %s HTTPS endpoint", expectedHost)
	}
	return nil
}

type emailAdapter struct {
	address    string
	host       string
	port       int
	username   string
	password   string
	from       string
	recipients []string
	transport  SMTPTransport
}

func newEmailAdapter(configuration Configuration, transport SMTPTransport) (Adapter, error) {
	if strings.TrimSpace(configuration.SMTPHost) == "" || configuration.SMTPPort < 1 || configuration.SMTPPort > 65535 {
		return nil, errors.New("SMTP host and port are required")
	}
	sender, err := mail.ParseAddress(configuration.From)
	if err != nil {
		return nil, errors.New("invalid sender address")
	}
	if len(configuration.Recipients) == 0 {
		return nil, errors.New("at least one recipient is required")
	}
	for _, recipient := range configuration.Recipients {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return nil, errors.New("invalid recipient address")
		}
	}
	return emailAdapter{
		address:    configuration.SMTPHost + ":" + strconv.Itoa(configuration.SMTPPort),
		host:       configuration.SMTPHost,
		port:       configuration.SMTPPort,
		username:   configuration.Username,
		password:   configuration.Password,
		from:       sender.Address,
		recipients: append([]string(nil), configuration.Recipients...),
		transport:  transport,
	}, nil
}

func (a emailAdapter) Deliver(ctx context.Context, message Message) error {
	if strings.ContainsAny(message.Subject, "\r\n") {
		return errors.New("invalid email subject")
	}
	authentication := smtp.Auth(nil)
	if a.username != "" {
		authentication = smtp.PlainAuth("", a.username, a.password, a.host)
	}
	headers := []string{
		"From: " + a.from,
		"To: " + strings.Join(a.recipients, ", "),
		"Subject: " + mime.QEncoding.Encode("UTF-8", message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		strings.ReplaceAll(message.Body, "\n", "\r\n"),
	}
	return a.transport.Send(ctx, a.address, a.host, a.port, authentication, a.from, a.recipients, []byte(strings.Join(headers, "\r\n")))
}

type smtpTransport struct{}

func (smtpTransport) Send(ctx context.Context, address, host string, port int, authentication smtp.Auth, from string, recipients []string, message []byte) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("SMTP connection failed")
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	}
	tlsConfiguration := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if port == 465 {
		tlsConnection := tls.Client(connection, tlsConfiguration)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			connection.Close()
			return errors.New("SMTP TLS handshake failed")
		}
		connection = tlsConnection
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		connection.Close()
		return errors.New("SMTP greeting failed")
	}
	defer client.Close()
	if port != 465 {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return errors.New("SMTP server does not require STARTTLS")
		}
		if err := client.StartTLS(tlsConfiguration); err != nil {
			return errors.New("SMTP STARTTLS failed")
		}
	}
	if authentication != nil {
		if err := client.Auth(authentication); err != nil {
			return errors.New("SMTP authentication failed")
		}
	}
	if err := client.Mail(from); err != nil {
		return errors.New("SMTP sender was rejected")
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return errors.New("SMTP recipient was rejected")
		}
	}
	writer, err := client.Data()
	if err != nil {
		return errors.New("SMTP message was rejected")
	}
	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return errors.New("SMTP message upload failed")
	}
	if err := writer.Close(); err != nil {
		return errors.New("SMTP message completion failed")
	}
	if err := client.Quit(); err != nil {
		return errors.New("SMTP session completion failed")
	}
	return nil
}
