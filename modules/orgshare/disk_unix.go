// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package orgshare

import (
	"errors"
	"math"

	"golang.org/x/sys/unix"
)

func measureVolume(path string) (Volume, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return Volume{}, err
	}
	if stats.Bsize <= 0 {
		return Volume{}, errors.New("invalid volume statistics")
	}
	blockSize := uint64(stats.Bsize)
	limit := uint64(math.MaxInt64) / blockSize
	if stats.Blocks > limit || stats.Bavail > limit {
		return Volume{}, errors.New("volume statistics overflow")
	}
	return Volume{
		CapacityBytes:  int64(stats.Blocks * blockSize),
		AvailableBytes: int64(stats.Bavail * blockSize),
	}, nil
}
