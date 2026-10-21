// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSQLiteConnectionDurability(t *testing.T) {
	for _, journal := range []string{"DELETE", "WAL"} {
		for _, mode := range []string{"", "FULL", "NORMAL"} {
			t.Run(journal+"/"+mode, func(t *testing.T) {
				driver, source, err := makeSQLiteConnStr(SQLiteConnStrOptions{
					FilePath:    filepath.Join(t.TempDir(), "durability.db"),
					BusyTimeout: 5000, JournalMode: journal, Synchronous: mode,
				})
				if err != nil {
					t.Fatal(err)
				}
				database, err := sql.Open(driver, source)
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				expected := 2
				if mode == "NORMAL" {
					expected = 1
				} else if mode == "" {
					baseline, err := sql.Open(driver, filepath.Join(t.TempDir(), "driver-default.db"))
					if err != nil {
						t.Fatal(err)
					}
					defer baseline.Close()
					if err := baseline.QueryRowContext(t.Context(), "PRAGMA synchronous").Scan(&expected); err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					connection, err := database.Conn(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					defer connection.Close()
					var actual int
					if err := connection.QueryRowContext(t.Context(), "PRAGMA synchronous").Scan(&actual); err != nil {
						t.Fatal(err)
					}
					if actual != expected {
						t.Fatalf("each connection must retain requested durability: %s/%s is %d, expected %d", journal, mode, actual, expected)
					}
				}
			})
		}
	}
	if _, _, err := makeSQLiteConnStr(SQLiteConnStrOptions{Synchronous: "not-a-mode"}); err == nil {
		t.Fatal("invalid durability must fail before opening a database")
	}
}
