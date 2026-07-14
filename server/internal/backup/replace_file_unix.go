// Copyright 2026 gitsss Authors. All rights reserved.

//go:build !windows

package backup

import "os"

func replaceCutoverFile(source, destination string) error {
	return os.Rename(source, destination)
}
