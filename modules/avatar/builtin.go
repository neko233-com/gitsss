// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"embed"
	"errors"
	"fmt"
)

//go:embed builtin/*.png
var builtinAssets embed.FS

const BuiltinCount = 24

// BuiltinPNG returns a locally embedded, immutable avatar; no external service is needed.
func BuiltinPNG(index int) ([]byte, error) {
	if index < 0 || index >= BuiltinCount {
		return nil, errors.New("invalid built-in avatar")
	}
	return builtinAssets.ReadFile(fmt.Sprintf("builtin/%02d.png", index))
}
