package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeBuiltinAction(t *testing.T) {
	root := t.TempDir()
	for repo, version := range map[string]string{
		"checkout": "v4", "setup-node": "v7", "setup-go": "v6", "cache": "v6",
		"upload-artifact": "v7", "download-artifact": "v8",
	} {
		source := filepath.Join(root, "actions", repo, version)
		require.NoError(t, os.MkdirAll(filepath.Join(source, "dist"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(source, "action.yml"), []byte("name: "+repo+"\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(source, "dist", "index.js"), []byte("// bundled "+repo+"\n"), 0o644))
	}
	for _, subaction := range []string{"cache/v6/restore", "cache/v6/save", "upload-artifact/v7/merge"} {
		source := filepath.Join(root, "actions", filepath.FromSlash(subaction))
		require.NoError(t, os.MkdirAll(source, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(source, "action.yml"), []byte("name: subaction\n"), 0o644))
	}
	t.Setenv("GITSSS_BUILTIN_ACTIONS_DIR", root)

	for _, action := range []string{
		"actions/checkout@v4",
		"actions/setup-node@v7",
		"actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38",
		"actions/setup-go@v6",
		"actions/cache@v4",
		"actions/cache/restore@v6",
		"actions/cache/save@55cc8345863c7cc4c66a329aec7e433d2d1c52a9",
		"actions/upload-artifact@v7",
		"actions/upload-artifact/merge@v6",
		"actions/download-artifact@v8",
	} {
		requestedDir := filepath.Join(t.TempDir(), "cache", strings.ReplaceAll(action, "/", "-"))
		actionDir, ok, err := materializeBuiltinAction(newRemoteAction(action), requestedDir)
		require.NoError(t, err)
		assert.True(t, ok, action)
		assert.Contains(t, actionDir, "-builtin-")
		assert.NotEqual(t, requestedDir, actionDir)
		_, err = os.Stat(filepath.Join(actionDir, "dist", "index.js"))
		require.NoError(t, err)
	}

	for _, action := range []string{
		"actions/checkout@v3",
		"actions/cache/delete@v6",
		"actions/upload-artifact/unknown@v7",
		"actions/download-artifact@v3",
		"https://github.com/actions/checkout@v4",
	} {
		requestedDir := filepath.Join(t.TempDir(), "action")
		actionDir, ok, err := materializeBuiltinAction(newRemoteAction(action), requestedDir)
		require.NoError(t, err)
		assert.False(t, ok, action)
		assert.Equal(t, requestedDir, actionDir)
	}
}
