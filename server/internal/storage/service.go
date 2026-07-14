// Copyright 2026 gitsss Authors. All rights reserved.

package storage

import (
	"sort"
	"sync"
)

type Category string

const (
	CategoryGitObjects Category = "git_objects"
	CategoryLFSAssets  Category = "lfs_assets"
	CategoryArtifacts  Category = "artifacts"
	CategoryBackups    Category = "backups"
)

type Usage struct {
	Category Category `json:"category"`
	Bytes    int64    `json:"bytes"`
}

type Snapshot struct {
	CapacityBytes  int64   `json:"capacity_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	UsageRatio     float64 `json:"usage_ratio"`
	Warning        bool    `json:"warning"`
	WriteProtected bool    `json:"write_protected"`
	Usage          []Usage `json:"usage"`
}

type Admission struct {
	Allowed        bool   `json:"allowed"`
	WriteProtected bool   `json:"write_protected"`
	Reason         string `json:"reason,omitempty"`
}

type Service struct {
	mu       sync.RWMutex
	capacity int64
	usage    map[Category]int64
}

func NewService(capacity int64, initialUsage []Usage) *Service {
	if capacity < 1 {
		panic("storage capacity must be positive")
	}

	service := &Service{capacity: capacity, usage: make(map[Category]int64)}
	for _, item := range initialUsage {
		service.SetUsage(item.Category, item.Bytes)
	}
	return service
}

func (s *Service) SetUsage(category Category, bytes int64) {
	if bytes < 0 {
		panic("storage usage cannot be negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage[category] = bytes
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	usage := make([]Usage, 0, len(s.usage))
	var used int64
	for category, bytes := range s.usage {
		usage = append(usage, Usage{Category: category, Bytes: bytes})
		used += bytes
	}
	sort.Slice(usage, func(i, j int) bool { return usage[i].Category < usage[j].Category })

	available := s.capacity - used
	if available < 0 {
		available = 0
	}
	ratio := float64(used) / float64(s.capacity)
	return Snapshot{
		CapacityBytes:  s.capacity,
		UsedBytes:      used,
		AvailableBytes: available,
		UsageRatio:     ratio,
		Warning:        ratio >= 0.85,
		WriteProtected: ratio >= 0.95,
		Usage:          usage,
	}
}

func (s *Service) Admit(writeBytes int64) Admission {
	if writeBytes < 0 {
		return Admission{Reason: "write size cannot be negative"}
	}

	snapshot := s.Snapshot()
	projectedUsage := snapshot.UsedBytes + writeBytes
	if projectedUsage > snapshot.CapacityBytes {
		return Admission{Reason: "insufficient capacity"}
	}
	if projectedUsage*100 >= snapshot.CapacityBytes*95 {
		return Admission{WriteProtected: true, Reason: "projected usage reaches the protected threshold"}
	}
	return Admission{Allowed: true}
}
