// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"strings"
	"testing"
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
