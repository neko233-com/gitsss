// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrivateConfigRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config.json")
	want := Config{Schedule: Schedule{Time: "21:00"}, Accounts: map[string]string{"fixture": "person"}, APIKey: "test-only"}
	if err := SaveConfig(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil || got.APIKey != want.APIKey || got.Accounts["fixture"] != "person" {
		t.Fatalf("round trip failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file permissions: %v", info.Mode())
	}
	want.Accounts["fixture"] = "unknown"
	if err := SaveConfig(path, want); err == nil {
		t.Fatal("unknown account type accepted")
	}
	got, err = LoadConfig(path)
	if err != nil || got.Accounts["fixture"] != "person" {
		t.Fatal("invalid update changed saved configuration")
	}
}

func TestCalendarValidation(t *testing.T) {
	for _, s := range []Schedule{
		{Time: "21:00", Holidays: []string{"2026-10-01"}},
		{Time: "21:00", CalendarYears: []int{2026, 2026}},
		{Time: "21:00", CalendarYears: []int{2026}, Holidays: []string{"2026-10-01"}, Workdays: []string{"2026-10-01"}},
	} {
		if err := (Config{Schedule: s}).Validate(); err == nil {
			t.Fatalf("accepted invalid calendar: %+v", s)
		}
	}
}
