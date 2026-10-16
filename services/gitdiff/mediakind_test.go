// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import "testing"

func TestMediaKindSkipsByteDiff(t *testing.T) {
	cases := map[string]string{
		"hero.PNG":     "image",
		"voice/a.wav":  "audio",
		"cutscene.mp4": "video",
		"prop.glb":     "model",
		"notes.txt":    "",
		"archive.bin":  "",
	}
	for name, want := range cases {
		if got := MediaKind(name); got != want {
			t.Fatalf("%s: got %q want %q", name, got, want)
		}
	}
}
