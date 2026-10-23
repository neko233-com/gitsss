// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// WriteAtReader writes length bytes from r at offset. The offset does not have to match the current size,
// so several chunks of one upload can land at the same time.
func WriteAtReader(filePath string, offset int64, r io.Reader, length int64) error {
	if offset < 0 || length < 0 {
		return errors.New("upload offset must not be negative")
	}
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if length == 0 {
		return nil
	}
	buf := make([]byte, 1<<20)
	written, err := io.CopyBuffer(io.NewOffsetWriter(file, offset), io.LimitReader(r, length), buf)
	if err != nil {
		return err
	}
	if written != length {
		return fmt.Errorf("short write: %d of %d", written, length)
	}
	return nil
}

// WriteAt writes one chunk at an arbitrary offset.
func WriteAt(filePath string, offset int64, data []byte) (int64, error) {
	if err := WriteAtReader(filePath, offset, bytes.NewReader(data), int64(len(data))); err != nil {
		return 0, err
	}
	return offset + int64(len(data)), nil
}

// Reserve creates the upload file and asks the filesystem to keep size bytes when it can.
// The boolean is false when the file is only sparse, so later chunks must recheck free space.
func Reserve(filePath string, size int64) (bool, error) {
	if size < 0 {
		return false, errors.New("upload size must not be negative")
	}
	return reserveSpace(filePath, size)
}

// Open returns the stored object for streaming. The caller closes it.
func Open(filePath string) (*os.File, error) {
	return os.Open(filePath)
}
