// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import "strings"

// MediaKind classifies a game asset for side-by-side preview.
// Binary bytes are not compared. An empty result means there is no preview.
func MediaKind(name string) string {
	ext := strings.ToLower(name)
	if i := strings.LastIndex(ext, "."); i >= 0 {
		ext = ext[i:]
	} else {
		return ""
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".bmp", ".tga", ".dds", ".svg":
		return "image"
	case ".wav", ".ogg", ".mp3", ".flac", ".aac", ".m4a":
		return "audio"
	case ".mp4", ".webm", ".mov", ".mkv":
		return "video"
	case ".glb", ".gltf", ".obj", ".fbx", ".stl":
		return "model"
	default:
		return ""
	}
}
