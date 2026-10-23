// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package orgshare

import (
	"errors"
	"math"
	"os"

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

func reserveSpace(path string, size int64) (bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if size == 0 {
		return true, nil
	}
	store := unix.Fstore_t{Flags: unix.F_ALLOCATEALL, Posmode: unix.F_PEOFPOSMODE, Length: size}
	err = unix.FcntlFstore(file.Fd(), unix.F_PREALLOCATE, &store)
	if err == nil {
		if err := file.Truncate(size); err != nil {
			return false, err
		}
		return true, nil
	}
	// Volumes without allocation support fall back to a sparse file.
	if err := file.Truncate(size); err != nil {
		return false, err
	}
	return false, nil
}
