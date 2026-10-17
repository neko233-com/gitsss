// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvatarStyleCatalogAndInitials(t *testing.T) {
	seen := map[int]bool{}
	for _, style := range BuiltinStyles() {
		assert.NotEmpty(t, style.Label)
		for _, index := range style.Indexes {
			assert.False(t, seen[index])
			seen[index] = true
		}
	}
	assert.Len(t, seen, 30)
	assert.Equal(t, "AB", UsernameInitials("alice-bob"))
	assert.Equal(t, "AL", UsernameInitials("alice"))
	for palette := range 8 {
		data, err := UsernameAvatarPNG("alice-bob", palette)
		require.NoError(t, err)
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		require.NoError(t, err)
		assert.Equal(t, 256, cfg.Width)
		assert.Equal(t, 256, cfg.Height)
		second, err := UsernameAvatarPNG("alice-bob", palette)
		require.NoError(t, err)
		assert.Equal(t, data, second)
	}
	_, err := UsernameAvatarPNG("alice", 8)
	assert.Error(t, err)
}
