// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build external_builtin_actions

package builtinactions

import "io/fs"

// Files is empty in images that already install the resources at the legacy path.
var Files fs.FS
