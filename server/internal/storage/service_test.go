// Copyright 2026 gitsss Authors. All rights reserved.

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceReportsUsageAndProtectsWrites(t *testing.T) {
	service := NewService(100, []Usage{{Category: CategoryGitObjects, Bytes: 40}, {Category: CategoryLFSAssets, Bytes: 47}})

	snapshot := service.Snapshot()
	if snapshot.AvailableBytes != 13 || !snapshot.Warning || snapshot.WriteProtected {
		t.Fatalf("snapshot = %#v, want warning with 13 bytes available", snapshot)
	}
	if admission := service.Admit(8); admission.Allowed || !admission.WriteProtected {
		t.Fatalf("admission = %#v, want protected write", admission)
	}
	if admission := service.Admit(14); admission.Allowed || admission.Reason != "insufficient capacity" {
		t.Fatalf("admission = %#v, want capacity rejection", admission)
	}
	plan := service.CleanupPlan()
	if plan.Status != "warning" || plan.BytesToTarget != 2 || plan.AutomaticDelete || len(plan.Actions) != 2 || plan.Actions[0].Kind != "move_lfs" {
		t.Fatalf("cleanup plan = %#v", plan)
	}
}

func TestCleanupPlanFailsClosedAndPrioritizesReviewableData(t *testing.T) {
	service := NewService(100, []Usage{
		{Category: CategoryBackups, Bytes: 20},
		{Category: CategoryArtifacts, Bytes: 30},
		{Category: CategoryDatabase, Bytes: 46},
	})
	plan := service.CleanupPlan()
	if plan.Status != "protected" || plan.BytesToTarget != 11 || len(plan.Actions) != 2 {
		t.Fatalf("cleanup plan = %#v", plan)
	}
	if plan.Actions[0].Kind != "review_expired_artifacts" || plan.Actions[0].ReviewBytes != 30 || plan.Actions[1].Kind != "review_redundant_backups" {
		t.Fatalf("cleanup actions = %#v", plan.Actions)
	}

	root := t.TempDir()
	live, err := newLiveService(LiveOptions{VolumePath: root, CacheTTL: time.Second}, func(string) (VolumeStats, error) {
		return VolumeStats{CapacityBytes: 100, AvailableBytes: 50}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	live.cacheExpiry = time.Time{}
	live.live.provider = func(string) (VolumeStats, error) { return VolumeStats{}, errors.New("volume offline") }
	unavailable := live.CleanupPlan()
	if unavailable.Status != "unavailable" || unavailable.BytesToTarget != 0 || len(unavailable.Actions) != 1 || unavailable.Actions[0].Kind != "restore_measurement" {
		t.Fatalf("unavailable plan = %#v", unavailable)
	}
}

func TestLiveServiceMeasuresVolumeAndAttributesGameAssets(t *testing.T) {
	root := t.TempDir()
	assets := map[string][]byte{
		"concept.psd": make([]byte, 101),
		"level.fbx":   make([]byte, 202),
		"music.mp3":   make([]byte, 303),
		"trailer.mp4": make([]byte, 404),
	}
	for name, content := range assets {
		if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stats := VolumeStats{CapacityBytes: 1_000, AvailableBytes: 200}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service, err := newLiveService(LiveOptions{
		VolumePath: root, ReserveBytes: 50,
		InventoryRoots: []InventoryRoot{{Category: CategoryLFSAssets, Path: root}}, CacheTTL: time.Second,
	}, func(string) (VolumeStats, error) { return stats, nil })
	if err != nil {
		t.Fatal(err)
	}
	service.live.now = func() time.Time { return now }
	service.cacheExpiry = time.Time{}
	snapshot := service.Snapshot()
	if snapshot.Source != "live" || snapshot.CapacityBytes != 1_000 || snapshot.AvailableBytes != 200 || snapshot.UsedBytes != 800 || snapshot.ReservedBytes != 50 {
		t.Fatalf("live snapshot = %#v", snapshot)
	}
	if len(snapshot.GameAssets) != 4 || snapshot.GameAssets[0].Extension != ".mp4" || snapshot.GameAssets[0].Strategy != "lfs_no_recompress" {
		t.Fatalf("game assets = %#v", snapshot.GameAssets)
	}
	if usage := usageFor(snapshot, CategoryLFSAssets); usage.Bytes != 1_010 || usage.Files != 4 {
		t.Fatalf("LFS usage = %#v", usage)
	}
	if admission := service.Admit(151); admission.Allowed || !admission.WriteProtected {
		t.Fatalf("reserve admission = %#v", admission)
	}
	if admission := service.Admit(201); admission.Allowed || admission.Reason != "insufficient capacity" {
		t.Fatalf("capacity admission = %#v", admission)
	}
}

func TestLiveServiceFailsClosedWhenVolumeMeasurementFails(t *testing.T) {
	root := t.TempDir()
	failure := false
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	service, err := newLiveService(LiveOptions{VolumePath: root, CacheTTL: time.Second}, func(string) (VolumeStats, error) {
		if failure {
			return VolumeStats{}, errors.New("volume offline")
		}
		return VolumeStats{CapacityBytes: 1_000, AvailableBytes: 500}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service.live.now = func() time.Time { return now }
	service.cacheExpiry = time.Time{}
	if snapshot := service.Snapshot(); snapshot.MeasurementError != "" {
		t.Fatalf("initial snapshot = %#v", snapshot)
	}
	failure = true
	now = now.Add(2 * time.Second)
	snapshot := service.Snapshot()
	if snapshot.MeasurementError != "volume offline" || !snapshot.WriteProtected || !snapshot.Warning {
		t.Fatalf("failed snapshot = %#v", snapshot)
	}
	if admission := service.Admit(1); admission.Allowed || !admission.WriteProtected || admission.Reason != "storage capacity could not be measured" {
		t.Fatalf("failed admission = %#v", admission)
	}
}

func TestLiveServiceBoundsInventoryAndReadsActualVolume(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one.psd", "two.fbx", "three.mp4"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	service, err := newLiveService(LiveOptions{
		VolumePath: root, InventoryRoots: []InventoryRoot{{Category: CategoryLFSAssets, Path: root}}, MaxEntries: 2,
	}, func(string) (VolumeStats, error) { return VolumeStats{CapacityBytes: 1_000, AvailableBytes: 500}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if snapshot := service.Snapshot(); !snapshot.InventoryTruncated || usageFor(snapshot, CategoryLFSAssets).Files != 2 {
		t.Fatalf("bounded inventory = %#v", snapshot)
	}
	actual, err := volumeStats(root)
	if err != nil {
		t.Fatal(err)
	}
	if actual.CapacityBytes <= 0 || actual.AvailableBytes <= 0 || actual.AvailableBytes > actual.CapacityBytes {
		t.Fatalf("actual volume stats = %#v", actual)
	}
}

func usageFor(snapshot Snapshot, category Category) Usage {
	for _, usage := range snapshot.Usage {
		if usage.Category == category {
			return usage
		}
	}
	return Usage{Category: category}
}
