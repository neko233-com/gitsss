// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/require"
)

func TestUploadedAvatarAlwaysCropsAndBoundsDimensions(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()
	defer test.MockVariableValue(&setting.Avatar.RenderedSizeFactor, 2)()
	for _, size := range []image.Point{{X: 1200, Y: 800}, {X: 100, Y: 60}, {X: 800, Y: 1200}} {
		img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
		for y := range size.Y {
			for x := range size.X {
				img.SetRGBA(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), A: 255})
			}
		}
		var input bytes.Buffer
		require.NoError(t, png.Encode(&input, img))
		result, err := NormalizeUploadedAvatar(input.Bytes())
		require.NoError(t, err)
		cfg, _, err := image.DecodeConfig(bytes.NewReader(result))
		require.NoError(t, err)
		require.Equal(t, min(512, size.X, size.Y), cfg.Width)
		require.Equal(t, cfg.Width, cfg.Height)
	}
}

func TestUploadedAvatarPreservesTransparencyAndRejectsSVG(t *testing.T) {
	defer test.MockVariableValue(&setting.Avatar.MaxWidth, 4096)()
	defer test.MockVariableValue(&setting.Avatar.MaxHeight, 4096)()
	img := image.NewRGBA(image.Rect(0, 0, 80, 60))
	var input bytes.Buffer
	require.NoError(t, png.Encode(&input, img))
	data, err := NormalizeUploadedAvatar(input.Bytes())
	require.NoError(t, err)
	_, format, err := image.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	_, err = NormalizeUploadedAvatar([]byte(`<svg onload="alert(1)"></svg>`))
	require.Error(t, err)
}
