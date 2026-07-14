// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/storage"
)

func TestLiveStorageOptionsUseDataVolumeAndAssetRoots(t *testing.T) {
	root := t.TempDir()
	dataPath := filepath.Join(root, "state", "gitsss.db")
	repositoryPath := filepath.Join(root, "repositories")
	lfsOne := filepath.Join(root, "lfs-one")
	lfsTwo := filepath.Join(root, "lfs-two")
	t.Setenv("GITSSS_STORAGE_RESERVE_BYTES", "4096")
	t.Setenv("GITSSS_STORAGE_MAX_ENTRIES", "2048")
	t.Setenv("GITSSS_STORAGE_CACHE_TTL", "45s")
	t.Setenv("GITSSS_LFS_PATHS", lfsOne+string(os.PathListSeparator)+lfsTwo)
	options, err := liveStorageOptions(dataPath, repositoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if options.VolumePath != filepath.Dir(dataPath) || options.ReserveBytes != 4096 || options.MaxEntries != 2048 || options.CacheTTL != 45*time.Second {
		t.Fatalf("storage options = %#v", options)
	}
	if len(options.InventoryRoots) != 4 || options.InventoryRoots[2].Category != storage.CategoryLFSAssets || options.InventoryRoots[3].Path != lfsTwo {
		t.Fatalf("inventory roots = %#v", options.InventoryRoots)
	}
}

func TestLiveStorageOptionsRejectInvalidLimits(t *testing.T) {
	t.Setenv("GITSSS_STORAGE_RESERVE_BYTES", "-1")
	if _, err := liveStorageOptions("data/gitsss.db", "."); err == nil {
		t.Fatal("negative reserve was accepted")
	}
	t.Setenv("GITSSS_STORAGE_RESERVE_BYTES", "1")
	t.Setenv("GITSSS_STORAGE_CACHE_TTL", "11m")
	if _, err := liveStorageOptions("data/gitsss.db", "."); err == nil {
		t.Fatal("excessive cache TTL was accepted")
	}
}
