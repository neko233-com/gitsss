//go:build sqlite && !cgo

// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"database/sql"

	moderncsqlite "modernc.org/sqlite"
)

// modernc.org/sqlite is a pure-Go SQLite driver (no CGO required), which
// registers the database/sql driver under the name "sqlite". The compatibility
// alias below keeps the existing sqlite3 driver name used by XORM and tests.
func init() {
	sql.Register("sqlite3", &moderncsqlite.Driver{})
	EnableSQLite3 = true
	SupportedDatabaseTypes = append(SupportedDatabaseTypes, "sqlite3")
}
