// Copyright 2026 gitsss Authors. All rights reserved.

package state

import (
	"fmt"
	"os"
)

func ProtectFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("protected path must be a regular file")
	}
	return protectFile(path)
}
