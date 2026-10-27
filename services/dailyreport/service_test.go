// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"strings"
	"testing"
	"time"
)

func TestComparisonIDsRejectsRefsAndMalformedURLs(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	oldID, newID, ok := comparisonIDs("owner/repo/compare/" + a + "..." + b)
	if !ok || oldID != a || newID != b {
		t.Fatal("valid comparison was rejected")
	}
	for _, value := range []string{"owner/repo/compare/main..." + b, "owner/repo/compare/" + a + "...--help", a + "..." + b} {
		if _, _, ok := comparisonIDs(value); ok {
			t.Fatalf("invalid comparison accepted: %s", value)
		}
	}
}

func TestReportTitleUsesBeijingDateAndDisplayName(t *testing.T) {
	date := time.Date(2026, 10, 26, 21, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if got := reportTitle("user", "昵称", date); got != "昵称 · 2026-10-26 周一" {
		t.Fatalf("display title: %s", got)
	}
	if got := reportTitle("user", "", date.AddDate(0, 0, 6)); got != "user · 2026-11-01 周日" {
		t.Fatalf("fallback title: %s", got)
	}
}
