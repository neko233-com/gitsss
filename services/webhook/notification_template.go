package webhook

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

const (
	// MaxNotificationTemplateSize bounds templates stored in webhook metadata.
	MaxNotificationTemplateSize = 16 * 1024
	maxNotificationOutputSize   = 64 * 1024
)

// NotificationTemplateData is the sandboxed data available to notification templates.
type NotificationTemplateData struct {
	Event   string
	Text    string
	Payload map[string]any
}

type notificationOutput struct{ bytes.Buffer }

func (w *notificationOutput) Write(data []byte) (int, error) {
	if w.Len()+len(data) > maxNotificationOutputSize {
		return 0, errors.New("rendered notification exceeds 64 KiB")
	}
	return w.Buffer.Write(data)
}

func (w *notificationOutput) WriteString(text string) (int, error) {
	return w.Write([]byte(text))
}

// ValidateNotificationTemplate checks syntax without executing the template.
func ValidateNotificationTemplate(source string) error {
	if len(source) > MaxNotificationTemplateSize {
		return fmt.Errorf("template exceeds %d bytes", MaxNotificationTemplateSize)
	}
	if strings.TrimSpace(source) == "" {
		return nil
	}
	_, err := template.New("notification").Funcs(notificationAliasFunctions(nil)).Option("missingkey=error").Parse(source)
	return err
}

func renderNotificationTemplate(source string, event webhook_module.HookEventType, defaultText string, payloadJSON []byte) (string, error) {
	if strings.TrimSpace(source) == "" {
		return defaultText, nil
	}
	if err := ValidateNotificationTemplate(source); err != nil {
		return "", err
	}

	payload := map[string]any{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", fmt.Errorf("decode notification payload: %w", err)
	}
	aliases := notificationAliases(event, defaultText, payload)
	tmpl, err := template.New("notification").Funcs(notificationAliasFunctions(aliases)).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}

	var output notificationOutput
	if err := tmpl.Execute(&output, NotificationTemplateData{
		Event:   string(event),
		Text:    defaultText,
		Payload: payload,
	}); err != nil {
		return "", fmt.Errorf("render notification template: %w", err)
	}
	if output.Len() > maxNotificationOutputSize {
		return "", errors.New("rendered notification exceeds 64 KiB")
	}
	return strings.TrimSpace(output.String()), nil
}
