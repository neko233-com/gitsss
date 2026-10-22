// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package orgshare

import (
	"errors"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

func measureVolume(path string) (Volume, error) {
	directory, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Volume{}, err
	}
	var available, capacity, free uint64
	if err := windows.GetDiskFreeSpaceEx(directory, &available, &capacity, &free); err != nil {
		return Volume{}, err
	}
	if capacity > math.MaxInt64 || available > math.MaxInt64 {
		return Volume{}, errors.New("volume statistics overflow")
	}
	return Volume{CapacityBytes: int64(capacity), AvailableBytes: int64(available)}, nil
}

func reserveSpace(path string, size int64) (bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if size > 0 {
		if err := file.Truncate(size); err != nil {
			return false, err
		}
	}
	return false, nil
}
