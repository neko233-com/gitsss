// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileDeliveryStorePersistsAndRejectsCorruption(t *testing.T) {
	store := FileDeliveryStore{Path: filepath.Join(t.TempDir(), "private", "deliveries.json")}
	if got, err := store.Delivered("2026-01-01", "fixture"); err != nil || got {
		t.Fatal("new store must be empty")
	}
	if err := store.MarkDelivered("2026-01-01", "fixture"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Delivered("2026-01-01", "fixture"); err != nil || !got {
		t.Fatal("receipt was not saved")
	}
	if err := os.WriteFile(store.Path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Delivered("2026-01-01", "fixture"); err == nil {
		t.Fatal("corrupt state accepted")
	}
}
