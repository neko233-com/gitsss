package webhook

import (
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/modules/json"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNotificationTemplate(t *testing.T) {
	payload := []byte(`{"repository":{"full_name":"poke-game/repo"}}`)

	rendered, err := renderNotificationTemplate(
		`[{{.Event}}] {{.Payload.repository.full_name}}: {{.Text}}`,
		webhook_module.HookEventPush,
		"updated",
		payload,
	)
	require.NoError(t, err)
	assert.Equal(t, "[push] poke-game/repo: updated", rendered)

	rendered, err = renderNotificationTemplate("", webhook_module.HookEventPush, "default", payload)
	require.NoError(t, err)
	assert.Equal(t, "default", rendered)
}

func TestNotificationTemplateRejectsUnsafeOutput(t *testing.T) {
	require.Error(t, ValidateNotificationTemplate("{{"))
	require.Error(t, ValidateNotificationTemplate(strings.Repeat("x", MaxNotificationTemplateSize+1)))

	_, err := renderNotificationTemplate(
		`{{.Payload.missing}}`,
		webhook_module.HookEventPush,
		"default",
		[]byte(`{"repository":{}}`),
	)
	require.Error(t, err)

	payload, err := json.Marshal(map[string]any{"large": strings.Repeat("x", 64*1024+1)})
	require.NoError(t, err)
	_, err = renderNotificationTemplate(
		`{{.Payload.large}}`,
		webhook_module.HookEventPush,
		"default",
		payload,
	)
	require.Error(t, err)
}
