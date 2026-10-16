// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/require"
)

func TestIsolatedSSHDoesNotRewriteSystemAuthorization(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "builtin"}[builtin], func(t *testing.T) {
			defer test.MockVariableValue(&setting.SSH, setting.SSH)()
			setting.SSH.Disabled = !builtin
			setting.SSH.StartBuiltinServer = builtin
			setting.SSH.CreateAuthorizedKeysFile = true
			setting.SSH.CreateAuthorizedPrincipalsFile = true
			setting.SSH.RootPath = t.TempDir()
			for _, name := range []string{"authorized_keys", "authorized_principals"} {
				require.NoError(t, os.WriteFile(filepath.Join(setting.SSH.RootPath, name), []byte("original source authorization\n"), 0o600))
			}
			require.NoError(t, RewriteAllPublicKeys(t.Context()))
			require.NoError(t, RewriteAllPrincipalKeys(t.Context()))
			for _, name := range []string{"authorized_keys", "authorized_principals"} {
				content, err := os.ReadFile(filepath.Join(setting.SSH.RootPath, name))
				require.NoError(t, err)
				require.Equal(t, "original source authorization\n", string(content))
			}
		})
	}
}
