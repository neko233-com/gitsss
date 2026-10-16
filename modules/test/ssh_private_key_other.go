// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package test

import "os"

// SecureSSHPrivateKey restricts a fixture key to its current owner for OpenSSH.
func SecureSSHPrivateKey(path string) error {
	return os.Chmod(path, 0o600)
}
