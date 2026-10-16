package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/neko233-com/gitsss/runner/pkg/builtinassets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeBuiltinActionFromServerResources(t *testing.T) {
	if _, err := os.Stat(defaultBuiltinActionsDir); err == nil {
		t.Skip("external Actions directory takes precedence")
	}
	t.Setenv("GITSSS_BUILTIN_ACTIONS_DIR", "")
	previous := builtinassets.Fallback()
	t.Cleanup(func() { builtinassets.SetFallback(previous) })
	builtinassets.SetFallback(fstest.MapFS{
		"actions/checkout/v4/action.yml":    &fstest.MapFile{Data: []byte("name: checkout\n")},
		"actions/checkout/v4/dist/index.js": &fstest.MapFile{Data: []byte("// embedded checkout\n")},
	})
	dir, ok, err := materializeBuiltinAction(newRemoteAction("actions/checkout@v4"), filepath.Join(t.TempDir(), "action"))
	require.NoError(t, err)
	require.True(t, ok)
	content, err := os.ReadFile(filepath.Join(dir, "dist", "index.js"))
	require.NoError(t, err)
	assert.Equal(t, "// embedded checkout\n", string(content))
}

func TestMaterializeBuiltinAction(t *testing.T) {
	root := t.TempDir()
	for repo, version := range map[string]string{
		"checkout": "v4", "setup-node": "v7", "setup-go": "v6", "cache": "v6",
		"upload-artifact": "v7", "download-artifact": "v8",
		"express233-upload": "v1",
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
		"gitsss/express233-upload@v1",
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
		"actions/express233-upload@v1",
		"gitsss/express233-upload@v2",
		"someone/express233-upload@v1",
		"https://github.com/actions/checkout@v4",
	} {
		requestedDir := filepath.Join(t.TempDir(), "action")
		actionDir, ok, err := materializeBuiltinAction(newRemoteAction(action), requestedDir)
		require.NoError(t, err)
		assert.False(t, ok, action)
		assert.Equal(t, requestedDir, actionDir)
	}
}
