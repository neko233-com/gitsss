// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuiltinAvatarsAreLocalDistinctAndBounded(t *testing.T) {
	seen := make(map[[32]byte]bool)
	for i := range BuiltinCount {
		data, err := BuiltinPNG(i)
		require.NoError(t, err)
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		require.NoError(t, err)
		require.Equal(t, 256, cfg.Width)
		require.Equal(t, 256, cfg.Height)
		hash := sha256.Sum256(data)
		require.False(t, seen[hash])
		seen[hash] = true
	}
	_, err := BuiltinPNG(-1)
	require.Error(t, err)
	_, err = BuiltinPNG(BuiltinCount)
	require.Error(t, err)
}
