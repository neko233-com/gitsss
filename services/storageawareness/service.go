package storageawareness

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/models/db"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/storage/awareness"
	"github.com/neko233-com/gitsss/modules/storage/garbage"
)

var (
	serviceOnce sync.Once
	service     *awareness.Service
	serviceErr  error
	garbageOnce sync.Once
	garbageSvc  *garbage.Service
	garbageErr  error
)

type RepositoryUsage struct {
	ID         int64
	FullName   string
	GitBytes   int64
	LFSBytes   int64
	TotalBytes int64
}

func Get() (*awareness.Service, error) {
	serviceOnce.Do(func() {
		section := setting.CfgProvider.Section("storage_awareness")
		reserveBytes := section.Key("RESERVE_BYTES").MustInt64(10 << 30)
		cacheTTL := section.Key("CACHE_TTL").MustDuration(30 * time.Second)
		maxEntries := section.Key("MAX_ENTRIES").MustInt(100_000)

		roots := []awareness.InventoryRoot{}
		if setting.Database.Type.IsSQLite3() {
			roots = appendExisting(roots, awareness.CategoryDatabase, setting.Database.Path)
		}
		roots = appendExisting(roots, awareness.CategoryGitObjects, setting.RepoRootPath)
		roots = appendLocalStorage(roots, awareness.CategoryLFSAssets, setting.LFS.Storage)
		roots = appendLocalStorage(roots, awareness.CategoryArtifacts, setting.Actions.ArtifactStorage)
		roots = appendLocalStorage(roots, awareness.CategoryAttachments, setting.Attachment.Storage)
		roots = appendLocalStorage(roots, awareness.CategoryPackages, setting.Packages.Storage)
		roots = appendExisting(roots, awareness.CategoryBackups, filepath.Join(setting.AppDataPath, "backups"))

		service, serviceErr = awareness.NewLiveService(awareness.LiveOptions{
			VolumePath: setting.AppDataPath, ReserveBytes: reserveBytes,
			InventoryRoots: roots, CacheTTL: cacheTTL, MaxEntries: maxEntries,
		})
	})
	return service, serviceErr
}

func RepositoryUsages(ctx context.Context) ([]RepositoryUsage, error) {
	repositories := make([]*repo_model.Repository, 0)
	if err := db.GetEngine(ctx).Find(&repositories); err != nil {
		return nil, err
	}
	usages := make([]RepositoryUsage, 0, len(repositories))
	for _, repo := range repositories {
		usages = append(usages, RepositoryUsage{
			ID: repo.ID, FullName: repo.OwnerName + "/" + repo.Name,
			GitBytes: repo.GitSize, LFSBytes: repo.LFSSize, TotalBytes: repo.GitSize + repo.LFSSize,
		})
	}
	sort.Slice(usages, func(i, j int) bool {
		if usages[i].TotalBytes == usages[j].TotalBytes {
			return usages[i].FullName < usages[j].FullName
		}
		return usages[i].TotalBytes > usages[j].TotalBytes
	})
	return usages, nil
}

func Garbage() (*garbage.Service, error) {
	garbageOnce.Do(func() {
		section := setting.CfgProvider.Section("storage_awareness")
		tempAge := section.Key("GARBAGE_TEMP_MAX_AGE").MustDuration(24 * time.Hour)
		workAge := section.Key("GARBAGE_RUNNER_WORK_MAX_AGE").MustDuration(24 * time.Hour)
		cacheAge := section.Key("GARBAGE_RUNNER_CACHE_MAX_AGE").MustDuration(30 * 24 * time.Hour)
		garbageSvc, garbageErr = garbage.NewService([]garbage.Root{
			{Kind: "managed_temp", Path: setting.AppDataTempDir("").JoinPath(""), MaxAge: tempAge},
			{Kind: "runner_workdir", Path: filepath.Join(setting.CustomPath, "actions", "work"), MaxAge: workAge},
			{Kind: "runner_cache", Path: filepath.Join(setting.CustomPath, "actions", "cache"), MaxAge: cacheAge},
		})
	})
	return garbageSvc, garbageErr
}

func appendLocalStorage(roots []awareness.InventoryRoot, category awareness.Category, storage *setting.Storage) []awareness.InventoryRoot {
	if storage == nil || storage.Type != setting.LocalStorageType {
		return roots
	}
	return appendExisting(roots, category, storage.Path)
}

func appendExisting(roots []awareness.InventoryRoot, category awareness.Category, path string) []awareness.InventoryRoot {
	if path == "" {
		return roots
	}
	if _, err := os.Stat(path); err != nil {
		return roots
	}
	clean := filepath.Clean(path)
	for _, root := range roots {
		if root.Path == clean {
			return roots
		}
	}
	return append(roots, awareness.InventoryRoot{Category: category, Path: clean})
}
