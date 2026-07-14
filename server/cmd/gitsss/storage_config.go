// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/storage"
)

const defaultStorageReserveBytes = 10 << 30

func liveStorageOptions(dataPath string, repositoryPaths ...string) (storage.LiveOptions, error) {
	volumePath := strings.TrimSpace(os.Getenv("GITSSS_STORAGE_VOLUME_PATH"))
	if volumePath == "" {
		volumePath = filepath.Dir(dataPath)
	}
	reserve, err := integerEnvironment("GITSSS_STORAGE_RESERVE_BYTES", defaultStorageReserveBytes, 0)
	if err != nil {
		return storage.LiveOptions{}, err
	}
	maxEntries, err := integerEnvironment("GITSSS_STORAGE_MAX_ENTRIES", 100_000, 1)
	if err != nil {
		return storage.LiveOptions{}, err
	}
	cacheTTL := 30 * time.Second
	if configured := strings.TrimSpace(os.Getenv("GITSSS_STORAGE_CACHE_TTL")); configured != "" {
		cacheTTL, err = time.ParseDuration(configured)
		if err != nil || cacheTTL <= 0 || cacheTTL > 10*time.Minute {
			return storage.LiveOptions{}, fmt.Errorf("GITSSS_STORAGE_CACHE_TTL must be between 1ns and 10m")
		}
	}
	roots := []storage.InventoryRoot{{Category: storage.CategoryDatabase, Path: dataPath}}
	for _, repositoryPath := range repositoryPaths {
		if repositoryPath = strings.TrimSpace(repositoryPath); repositoryPath != "" {
			roots = append(roots, storage.InventoryRoot{Category: storage.CategoryGitObjects, Path: repositoryPath})
		}
	}
	for _, source := range []struct {
		environment string
		category    storage.Category
	}{
		{environment: "GITSSS_LFS_PATHS", category: storage.CategoryLFSAssets},
		{environment: "GITSSS_ARTIFACT_PATHS", category: storage.CategoryArtifacts},
		{environment: "GITSSS_BACKUP_PATHS", category: storage.CategoryBackups},
		{environment: "GITSSS_ATTACHMENT_PATHS", category: storage.CategoryAttachments},
	} {
		for _, path := range filepath.SplitList(os.Getenv(source.environment)) {
			if path = strings.TrimSpace(path); path != "" {
				roots = append(roots, storage.InventoryRoot{Category: source.category, Path: path})
			}
		}
	}
	return storage.LiveOptions{
		VolumePath: volumePath, ReserveBytes: reserve, InventoryRoots: roots,
		CacheTTL: cacheTTL, MaxEntries: int(maxEntries),
	}, nil
}

func integerEnvironment(name string, fallback, minimum int64) (int64, error) {
	configured := strings.TrimSpace(os.Getenv(name))
	if configured == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(configured, 10, 64)
	if err != nil || value < minimum {
		return 0, fmt.Errorf("%s must be an integer of at least %d", name, minimum)
	}
	return value, nil
}
