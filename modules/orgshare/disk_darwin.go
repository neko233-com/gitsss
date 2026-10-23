// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package orgshare

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func reserveSpace(path string, size int64) (bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if size == 0 {
		return true, nil
	}

	store := &unix.Fstore_t{
		Flags:   unix.F_ALLOCATECONTIG,
		Posmode: unix.F_PEOFPOSMODE,
		Length:  size,
	}
	err = unix.FcntlFstore(file.Fd(), unix.F_PREALLOCATE, store)
	if err != nil {
		store.Flags = unix.F_ALLOCATEALL
		err = unix.FcntlFstore(file.Fd(), unix.F_PREALLOCATE, store)
	}
	if err == nil {
		if err := file.Truncate(size); err != nil {
			return false, err
		}
		return true, nil
	}
	if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) {
		if err := file.Truncate(size); err != nil {
			return false, err
		}
		return false, nil
	}
	return false, err
}
