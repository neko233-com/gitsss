//go:build sqlite && !cgo

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	_ "modernc.org/sqlite"
)

// modernc.org/sqlite is a pure-Go SQLite driver (no CGO required), which
// registers the database/sql driver under the name "sqlite". The xorm
// dialect table already maps the "sqlite" driver name to the sqlite3 dialect,
// so no extra dialect registration is needed. This enables cross-platform
// (Linux/macOS/Windows, amd64/arm64) builds without a C toolchain.
func init() {
	EnableSQLite3 = true
	// Pure-Go build: modernc.org/sqlite registers the driver as "sqlite".
	SQLite3DriverName = "sqlite"
	SupportedDatabaseTypes = append(SupportedDatabaseTypes, "sqlite3")
}
