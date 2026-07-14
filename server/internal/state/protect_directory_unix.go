// Copyright 2026 gitsss Authors. All rights reserved.

//go:build !windows

package state

import "os"

func ProtectDirectory(path string) error {
	return os.Chmod(path, 0o700)
}
