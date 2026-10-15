// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repolayout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureAndExtensionDoNotRewriteGitData(t *testing.T) {
	root := t.TempDir()
	dataRoot := filepath.Join(root, "data")
	repoRoot := filepath.Join(dataRoot, "gitea-repositories")
	lfsRoot := filepath.Join(dataRoot, "lfs")
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "acme", "game.git", "objects", "pack"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "acme", "game.git", "refs", "heads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "acme", "game.git", "objects", "pack", "pack-test"), []byte("pack-bytes"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "acme", "game.git", "refs", "heads", "main"), []byte("abc123\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(lfsRoot, "aa", "bb"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(lfsRoot, "aa", "bb", "aabbcc"), []byte("lfs-bytes"), 0o644))

	beforeRepo, err := hashTree(repoRoot)
	require.NoError(t, err)
	beforeLFS, err := hashTree(lfsRoot)
	require.NoError(t, err)

	manifest, err := Ensure(dataRoot, repoRoot, lfsRoot)
	require.NoError(t, err)
	assert.Equal(t, Version, manifest.LayoutVersion)
	assert.Equal(t, ContractGitObjectsImmutable, manifest.Contract)
	assert.Equal(t, RequiredFrozen(), manifest.Frozen)

	dir, err := ReserveExtension(dataRoot, repoRoot, lfsRoot, "game-index")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(ExtensionsPath(dataRoot), "game-index"), dir)
	marker, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	require.NoError(t, err)
	assert.Contains(t, string(marker), "game-index")

	afterRepo, err := hashTree(repoRoot)
	require.NoError(t, err)
	afterLFS, err := hashTree(lfsRoot)
	require.NoError(t, err)
	assert.Equal(t, beforeRepo, afterRepo)
	assert.Equal(t, beforeLFS, afterLFS)

	_, err = ReserveExtension(dataRoot, repoRoot, lfsRoot, "../game")
	require.Error(t, err)
	_, err = ReserveExtension(dataRoot, repoRoot, lfsRoot, "Game")
	require.Error(t, err)
}

func TestFutureLayoutStopsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	dataRoot := filepath.Join(root, "data")
	repoRoot := filepath.Join(root, "repos")
	require.NoError(t, os.MkdirAll(filepath.Dir(ManifestPath(dataRoot)), 0o755))
	original := []byte("{\n  \"layout_version\": 99,\n  \"contract\": \"git-objects-immutable\",\n  \"future_field\": \"keep\"\n}\n")
	require.NoError(t, os.WriteFile(ManifestPath(dataRoot), original, 0o644))
	repoFile := filepath.Join(repoRoot, "acme", "game.git", "HEAD")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoFile), 0o755))
	require.NoError(t, os.WriteFile(repoFile, []byte("ref: refs/heads/main\n"), 0o644))
	before, err := hashTree(repoRoot)
	require.NoError(t, err)

	_, err = Ensure(dataRoot, repoRoot, "")
	require.ErrorIs(t, err, ErrFutureLayout)
	got, err := os.ReadFile(ManifestPath(dataRoot))
	require.NoError(t, err)
	assert.Equal(t, original, got)
	after, err := hashTree(repoRoot)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	_, err = os.Stat(ExtensionsPath(dataRoot))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsurePreservesUnknownKeysAndReservedExtensions(t *testing.T) {
	root := t.TempDir()
	dataRoot := filepath.Join(root, "data")
	repoRoot := filepath.Join(root, "repos")
	require.NoError(t, os.MkdirAll(filepath.Dir(ManifestPath(dataRoot)), 0o755))
	body, err := json.Marshal(map[string]any{
		"layout_version": 1,
		"contract":       ContractGitObjectsImmutable,
		"frozen":         []string{RoleRepositories},
		"extensions_dir": "gitsss/extensions",
		"future_field":   "keep",
		"reserved":       map[string]string{"audit": "gitsss/extensions/audit"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ManifestPath(dataRoot), body, 0o644))

	manifest, err := Ensure(dataRoot, repoRoot, "")
	require.NoError(t, err)
	assert.Equal(t, RequiredFrozen(), manifest.Frozen)
	assert.Equal(t, "gitsss/extensions/audit", manifest.Reserved["audit"])

	saved, err := os.ReadFile(ManifestPath(dataRoot))
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(saved, &raw))
	assert.Equal(t, "keep", raw["future_field"])
}

func TestEnsureRejectsExtensionOverlap(t *testing.T) {
	root := t.TempDir()
	dataRoot := filepath.Join(root, "data")
	_, err := Ensure(dataRoot, dataRoot, "")
	require.ErrorContains(t, err, "overlap")
	_, err = os.Stat(ManifestPath(dataRoot))
	require.ErrorIs(t, err, os.ErrNotExist)
}
