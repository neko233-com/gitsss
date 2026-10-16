// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package builtinassets accepts offline resources from the embedding server.
package builtinassets

import (
	"io/fs"
	"sync"
)

var fallback struct {
	sync.RWMutex
	fs fs.FS
}

// SetFallback supplies immutable resources without depending on the server module.
func SetFallback(files fs.FS) {
	fallback.Lock()
	defer fallback.Unlock()
	fallback.fs = files
}

// Fallback returns resources when no external Actions directory is installed.
func Fallback() fs.FS {
	fallback.RLock()
	defer fallback.RUnlock()
	return fallback.fs
}
