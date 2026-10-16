// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !external_builtin_actions

package builtinactions

import (
	"embed"
	"io/fs"
)

//go:embed actions
var assets embed.FS

// Files supplies the native binary's offline Actions resources.
var Files fs.FS = assets
