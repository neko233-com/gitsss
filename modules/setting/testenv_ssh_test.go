// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestEnvironmentIsolatesSSHAuthorization(t *testing.T) {
	defer test.MockVariableValue(&SSH, SSH)()
	cfg, err := NewConfigProviderFromData("[server]\nSSH_ROOT_PATH=/system/ssh\nSSH_CREATE_AUTHORIZED_KEYS_FILE=true\nSSH_CREATE_AUTHORIZED_PRINCIPALS_FILE=true\n")
	require.NoError(t, err)
	dataPath := t.TempDir()
	isolateTestSSH(cfg, dataPath)
	assert.Equal(t, filepath.Join(dataPath, "test-ssh"), SSH.RootPath)
	assert.False(t, SSH.CreateAuthorizedKeysFile)
	assert.False(t, SSH.CreateAuthorizedPrincipalsFile)
	assert.Equal(t, SSH.RootPath, cfg.Section("server").Key("SSH_ROOT_PATH").String())
	assert.Equal(t, "false", cfg.Section("server").Key("SSH_CREATE_AUTHORIZED_KEYS_FILE").String())
}
