// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !linux && !darwin && !windows

package orgshare

import "os"

func reserveSpace(path string, size int64) (bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := file.Truncate(size); err != nil {
		return false, err
	}
	return size == 0, nil
}
