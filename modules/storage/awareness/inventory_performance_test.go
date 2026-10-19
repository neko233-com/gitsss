// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package awareness

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "asset.psd"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, roots := range [][]InventoryRoot{
		{{CategoryGitObjects, root}, {CategoryLFSAssets, nested}},
		{{CategoryLFSAssets, nested}, {CategoryGitObjects, root}},
		{{CategoryGitObjects, root}, {CategoryLFSAssets, root}},
	} {
		usage, assets, truncated, err := scanInventories(roots, 100)
		if err != nil || truncated || len(usage) != 1 || usage[0].Bytes != 5 || usage[0].Files != 1 || usage[0].Category != roots[0].Category || len(assets) != 1 || assets[0].Files != 1 {
			t.Fatalf("inventory = %v %v %v %v", usage, assets, truncated, err)
		}
	}
}

func BenchmarkInventoryDisjointRoots(b *testing.B) {
	root := b.TempDir()
	roots := []InventoryRoot{{CategoryGitObjects, filepath.Join(root, "git")}, {CategoryLFSAssets, filepath.Join(root, "lfs")}}
	for _, item := range roots {
		if err := os.Mkdir(item.Path, 0o700); err != nil {
			b.Fatal(err)
		}
		for i := range 2000 {
			if err := os.WriteFile(filepath.Join(item.Path, fmt.Sprintf("object-%05d", i)), []byte("x"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		usage, _, truncated, err := scanInventories(roots, 100_000)
		if err != nil || truncated || len(usage) != 2 {
			b.Fatalf("inventory: %v %v", truncated, err)
		}
	}
}
