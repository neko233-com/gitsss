// Copyright 2026 gitsss Authors. All rights reserved.

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Category string

const (
	CategoryDatabase    Category = "database"
	CategoryGitObjects  Category = "git_objects"
	CategoryLFSAssets   Category = "lfs_assets"
	CategoryArtifacts   Category = "artifacts"
	CategoryBackups     Category = "backups"
	CategoryAttachments Category = "attachments"
	CategoryOther       Category = "other_volume_usage"
)

const (
	defaultCacheTTL   = 30 * time.Second
	defaultMaxEntries = 100_000
)

var ErrInvalidConfiguration = errors.New("invalid storage configuration")

type Usage struct {
	Category Category `json:"category"`
	Bytes    int64    `json:"bytes"`
	Files    int64    `json:"files,omitempty"`
}

type AssetUsage struct {
	Extension string `json:"extension"`
	Bytes     int64  `json:"bytes"`
	Files     int64  `json:"files"`
	Strategy  string `json:"strategy"`
}

type Snapshot struct {
	Source             string       `json:"source"`
	VolumePath         string       `json:"volume_path,omitempty"`
	CapacityBytes      int64        `json:"capacity_bytes"`
	UsedBytes          int64        `json:"used_bytes"`
	AvailableBytes     int64        `json:"available_bytes"`
	ReservedBytes      int64        `json:"reserved_bytes"`
	WarningFreeBytes   int64        `json:"warning_free_bytes"`
	ProtectedFreeBytes int64        `json:"protected_free_bytes"`
	UsageRatio         float64      `json:"usage_ratio"`
	Warning            bool         `json:"warning"`
	WriteProtected     bool         `json:"write_protected"`
	MeasurementError   string       `json:"measurement_error,omitempty"`
	InventoryError     string       `json:"inventory_error,omitempty"`
	InventoryTruncated bool         `json:"inventory_truncated"`
	MeasuredAt         time.Time    `json:"measured_at"`
	Usage              []Usage      `json:"usage"`
	GameAssets         []AssetUsage `json:"game_assets"`
}

type Admission struct {
	Allowed        bool   `json:"allowed"`
	WriteProtected bool   `json:"write_protected"`
	Reason         string `json:"reason,omitempty"`
}

type InventoryRoot struct {
	Category Category
	Path     string
}

type LiveOptions struct {
	VolumePath     string
	ReserveBytes   int64
	InventoryRoots []InventoryRoot
	CacheTTL       time.Duration
	MaxEntries     int
}

type VolumeStats struct {
	CapacityBytes  int64
	AvailableBytes int64
}

type volumeProvider func(string) (VolumeStats, error)

type liveConfiguration struct {
	path       string
	reserve    int64
	roots      []InventoryRoot
	cacheTTL   time.Duration
	maxEntries int
	provider   volumeProvider
	now        func() time.Time
}

type Service struct {
	mu          sync.Mutex
	capacity    int64
	usage       map[Category]Usage
	live        *liveConfiguration
	cached      Snapshot
	cacheExpiry time.Time
}

func NewService(capacity int64, initialUsage []Usage) *Service {
	if capacity < 1 {
		panic("storage capacity must be positive")
	}
	service := &Service{capacity: capacity, usage: make(map[Category]Usage)}
	for _, item := range initialUsage {
		service.SetUsage(item.Category, item.Bytes)
	}
	return service
}

func NewLiveService(options LiveOptions) (*Service, error) {
	return newLiveService(options, volumeStats)
}

func newLiveService(options LiveOptions, provider volumeProvider) (*Service, error) {
	path := strings.TrimSpace(options.VolumePath)
	if path == "" || options.ReserveBytes < 0 || options.ReserveBytes > math.MaxInt64/2 || provider == nil {
		return nil, ErrInvalidConfiguration
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	cacheTTL := options.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultCacheTTL
	}
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	roots := make([]InventoryRoot, 0, len(options.InventoryRoots))
	for _, root := range options.InventoryRoots {
		if !validCategory(root.Category) || strings.TrimSpace(root.Path) == "" {
			return nil, ErrInvalidConfiguration
		}
		rootPath, err := filepath.Abs(root.Path)
		if err != nil {
			return nil, err
		}
		roots = append(roots, InventoryRoot{Category: root.Category, Path: filepath.Clean(rootPath)})
	}
	service := &Service{usage: make(map[Category]Usage), live: &liveConfiguration{
		path: filepath.Clean(absolute), reserve: options.ReserveBytes, roots: roots,
		cacheTTL: cacheTTL, maxEntries: maxEntries, provider: provider, now: time.Now,
	}}
	snapshot := service.refreshLocked()
	if snapshot.MeasurementError != "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidConfiguration, snapshot.MeasurementError)
	}
	return service, nil
}

func (s *Service) SetUsage(category Category, bytes int64) {
	if bytes < 0 || !validCategory(category) {
		panic("storage usage is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage[category] = Usage{Category: category, Bytes: bytes}
	s.cacheExpiry = time.Time{}
}

func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.live == nil {
		return cloneSnapshot(s.staticSnapshotLocked())
	}
	now := s.live.now().UTC()
	if !s.cacheExpiry.IsZero() && now.Before(s.cacheExpiry) {
		return cloneSnapshot(s.cached)
	}
	return cloneSnapshot(s.refreshLocked())
}

func (s *Service) Admit(writeBytes int64) Admission {
	if writeBytes < 0 {
		return Admission{Reason: "write size cannot be negative"}
	}
	snapshot := s.Snapshot()
	if snapshot.MeasurementError != "" {
		return Admission{WriteProtected: true, Reason: "storage capacity could not be measured"}
	}
	if writeBytes > snapshot.AvailableBytes {
		return Admission{Reason: "insufficient capacity"}
	}
	if snapshot.AvailableBytes-writeBytes <= snapshot.ProtectedFreeBytes {
		return Admission{WriteProtected: true, Reason: "projected free space reaches the protected reserve"}
	}
	return Admission{Allowed: true}
}

func (s *Service) staticSnapshotLocked() Snapshot {
	usage := usageList(s.usage)
	var used int64
	for _, item := range usage {
		used = saturatingAdd(used, item.Bytes)
	}
	available := max(int64(0), s.capacity-used)
	return makeSnapshot("configured", "", s.capacity, available, 0, usage, nil, time.Now().UTC())
}

func (s *Service) refreshLocked() Snapshot {
	now := s.live.now().UTC()
	stats, err := s.live.provider(s.live.path)
	if err != nil || stats.CapacityBytes <= 0 || stats.AvailableBytes < 0 || stats.AvailableBytes > stats.CapacityBytes {
		message := "invalid volume statistics"
		if err != nil {
			message = err.Error()
		}
		s.cached = Snapshot{
			Source: "live", VolumePath: s.live.path, ReservedBytes: s.live.reserve,
			Warning: true, WriteProtected: true, MeasurementError: message, MeasuredAt: now,
			Usage: []Usage{}, GameAssets: []AssetUsage{},
		}
		s.cacheExpiry = now.Add(s.live.cacheTTL)
		return s.cached
	}
	usage, assets, truncated, inventoryErr := scanInventories(s.live.roots, s.live.maxEntries)
	physicalUsed := stats.CapacityBytes - stats.AvailableBytes
	var classified int64
	for _, item := range usage {
		classified = saturatingAdd(classified, item.Bytes)
	}
	if other := physicalUsed - classified; other > 0 {
		usage = append(usage, Usage{Category: CategoryOther, Bytes: other})
		sort.Slice(usage, func(left, right int) bool { return usage[left].Category < usage[right].Category })
	}
	snapshot := makeSnapshot("live", s.live.path, stats.CapacityBytes, stats.AvailableBytes, s.live.reserve, usage, assets, now)
	snapshot.InventoryTruncated = truncated
	if inventoryErr != nil {
		snapshot.InventoryError = inventoryErr.Error()
		snapshot.Warning = true
	}
	s.cached = snapshot
	s.cacheExpiry = now.Add(s.live.cacheTTL)
	return snapshot
}

func makeSnapshot(source, path string, capacity, available, reserve int64, usage []Usage, assets []AssetUsage, measuredAt time.Time) Snapshot {
	used := max(int64(0), capacity-available)
	warningFree := max(reserve*2, percentage(capacity, 15))
	protectedFree := max(reserve, percentage(capacity, 5))
	ratio := float64(used) / float64(capacity)
	return Snapshot{
		Source: source, VolumePath: path, CapacityBytes: capacity, UsedBytes: used,
		AvailableBytes: available, ReservedBytes: reserve, WarningFreeBytes: warningFree,
		ProtectedFreeBytes: protectedFree, UsageRatio: ratio, Warning: available <= warningFree,
		WriteProtected: available <= protectedFree, MeasuredAt: measuredAt,
		Usage: usage, GameAssets: assets,
	}
}

func scanInventories(roots []InventoryRoot, maxEntries int) ([]Usage, []AssetUsage, bool, error) {
	usage := make(map[Category]Usage)
	assets := make(map[string]AssetUsage)
	seen := make(map[string]struct{})
	entries := 0
	truncated := false
	var inventoryErr error
	for _, root := range roots {
		err := filepath.WalkDir(root.Path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			entries++
			if entries > maxEntries {
				truncated = true
				return fs.SkipAll
			}
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			absolute = filepath.Clean(absolute)
			if _, exists := seen[absolute]; exists {
				return nil
			}
			seen[absolute] = struct{}{}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			item := usage[root.Category]
			item.Category = root.Category
			item.Files++
			item.Bytes = saturatingAdd(item.Bytes, info.Size())
			usage[root.Category] = item
			extension := strings.ToLower(filepath.Ext(path))
			strategy := assetStrategy(extension)
			if strategy != "" {
				asset := assets[extension]
				asset.Extension = extension
				asset.Strategy = strategy
				asset.Files++
				asset.Bytes = saturatingAdd(asset.Bytes, info.Size())
				assets[extension] = asset
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.SkipAll) {
			inventoryErr = errors.Join(inventoryErr, fmt.Errorf("scan %s: %w", root.Path, err))
		}
		if truncated {
			break
		}
	}
	return usageList(usage), assetList(assets), truncated, inventoryErr
}

func usageList(values map[Category]Usage) []Usage {
	usage := make([]Usage, 0, len(values))
	for _, item := range values {
		usage = append(usage, item)
	}
	sort.Slice(usage, func(left, right int) bool { return usage[left].Category < usage[right].Category })
	return usage
}

func assetList(values map[string]AssetUsage) []AssetUsage {
	assets := make([]AssetUsage, 0, len(values))
	for _, item := range values {
		assets = append(assets, item)
	}
	sort.Slice(assets, func(left, right int) bool { return assets[left].Bytes > assets[right].Bytes })
	return assets
}

func assetStrategy(extension string) string {
	switch extension {
	case ".psd", ".psb", ".fbx", ".blend", ".wav", ".flac":
		return "chunked_lfs"
	case ".mp3", ".ogg", ".mp4", ".mov", ".mkv", ".webm":
		return "lfs_no_recompress"
	default:
		return ""
	}
}

func validCategory(category Category) bool {
	switch category {
	case CategoryDatabase, CategoryGitObjects, CategoryLFSAssets, CategoryArtifacts, CategoryBackups, CategoryAttachments, CategoryOther:
		return true
	default:
		return false
	}
}

func percentage(value, percent int64) int64 {
	if value > math.MaxInt64/percent {
		return value / 100 * percent
	}
	return value * percent / 100
}

func saturatingAdd(left, right int64) int64 {
	if right > 0 && left > math.MaxInt64-right {
		return math.MaxInt64
	}
	return left + right
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Usage = append([]Usage(nil), snapshot.Usage...)
	snapshot.GameAssets = append([]AssetUsage(nil), snapshot.GameAssets...)
	if snapshot.Usage == nil {
		snapshot.Usage = []Usage{}
	}
	if snapshot.GameAssets == nil {
		snapshot.GameAssets = []AssetUsage{}
	}
	return snapshot
}
