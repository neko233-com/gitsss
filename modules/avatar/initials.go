// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var initialsColors = []color.RGBA{
	{R: 37, G: 99, B: 235, A: 255},
	{R: 15, G: 118, B: 110, A: 255},
	{R: 124, G: 58, B: 237, A: 255},
	{R: 190, G: 24, B: 93, A: 255},
	{R: 194, G: 65, B: 12, A: 255},
	{R: 67, G: 56, B: 202, A: 255},
	{R: 4, G: 120, B: 87, A: 255},
	{R: 162, G: 28, B: 175, A: 255},
}

func UsernameInitials(name string) string {
	words := strings.FieldsFunc(strings.ToUpper(name), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(words) == 0 {
		return "?"
	}
	text := []rune(words[0])
	if len(words) > 1 {
		return string([]rune{text[0], []rune(words[1])[0]})
	}
	if len(text) > 2 {
		text = text[:2]
	}
	return string(text)
}

// UsernameAvatarPNG is deterministic, local and independent of third-party avatar services.
func UsernameAvatarPNG(name string, palette int) ([]byte, error) {
	if palette < 0 || palette >= len(initialsColors) || len(name) > 255 {
		return nil, errors.New("invalid username avatar")
	}
	typeface, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: 100, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.Draw(img, img.Bounds(), image.NewUniform(initialsColors[palette]), image.Point{}, draw.Src)
	text := UsernameInitials(name)
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: face}
	metrics := face.Metrics()
	d.Dot = fixed.Point26_6{X: (fixed.I(256) - d.MeasureString(text)) / 2, Y: (fixed.I(256) + metrics.Ascent - metrics.Descent) / 2}
	d.DrawString(text)
	var output bytes.Buffer
	err = png.Encode(&output, img)
	return output.Bytes(), err
}
