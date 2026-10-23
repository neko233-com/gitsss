// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAtArbitraryOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upload.bin")
	_, err := WriteAt(path, 5, []byte("!"))
	require.NoError(t, err)
	_, err = WriteAt(path, 0, []byte("hello"))
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello!", string(content))
}

func TestReserveAndMeasure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reserved.bin")
	preallocated, err := Reserve(path, 64*1024)
	require.NoError(t, err)
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		assert.True(t, preallocated)
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(64*1024), info.Size())

	volume, err := MeasureVolume(dir)
	require.NoError(t, err)
	assert.Positive(t, volume.CapacityBytes)
	assert.Positive(t, volume.AvailableBytes)
}
