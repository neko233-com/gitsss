//go:build windows

// Copyright 2026 gitsss Authors. All rights reserved.

package storage

import (
	"errors"
	"math"

	"golang.org/x/sys/windows"
)

func volumeStats(path string) (VolumeStats, error) {
	directory, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return VolumeStats{}, err
	}
	var available uint64
	var capacity uint64
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(directory, &available, &capacity, &free); err != nil {
		return VolumeStats{}, err
	}
	if capacity > math.MaxInt64 || available > math.MaxInt64 {
		return VolumeStats{}, errors.New("volume statistics overflow")
	}
	return VolumeStats{CapacityBytes: int64(capacity), AvailableBytes: int64(available)}, nil
}
