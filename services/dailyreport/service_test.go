// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/dailyreport"
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

func TestFilterRoleAccounts(t *testing.T) {
	makeGroups := func() map[string][]dailyreport.AttributedCommit {
		return map[string][]dailyreport.AttributedCommit{"client": {}, "designer": {}, "none": {}}
	}
	resolve := func(name string) ([]string, error) {
		switch name {
		case "client":
			return []string{"客户端", "后端"}, nil
		case "designer":
			return []string{"策划"}, nil
		default:
			return nil, nil
		}
	}
	groups := makeGroups()
	if err := filterRoleAccounts(groups, dailyreport.Config{}, resolve); err != nil || len(groups) != 3 {
		t.Fatal("all mode must retain every account")
	}
	groups = makeGroups()
	if err := filterRoleAccounts(groups, dailyreport.Config{RoleFilterEnabled: true, RoleLabels: []string{"客户端", "策划"}}, resolve); err != nil || len(groups) != 2 {
		t.Fatal("selected roles should match either label and exclude unlabeled users")
	}
	if _, ok := groups["none"]; ok {
		t.Fatal("unlabeled account included")
	}
	groups = makeGroups()
	if err := filterRoleAccounts(groups, dailyreport.Config{RoleFilterEnabled: true, RoleLabels: []string{"客户端"}}, func(string) ([]string, error) { return nil, errors.New("test") }); err == nil {
		t.Fatal("role lookup failures must stop the run")
	}
}
func TestHistoricalReportTimeUsesWholeBeijingDay(t *testing.T) {
	now := time.Date(2026, 11, 5, 2, 0, 0, 0, time.UTC)
	at, err := historicalReportTime("2026-11-04", now)
	if err != nil || at.Format(time.RFC3339) != "2026-11-04T23:59:59+08:00" {
		t.Fatalf("unexpected report cutoff %v: %v", at, err)
	}
	for _, value := range []string{"2026-11-05", "2026-11-06", "2026-11-4", "invalid"} {
		if _, err := historicalReportTime(value, now); err == nil {
			t.Fatalf("accepted invalid historical date %q", value)
		}
	}
}
