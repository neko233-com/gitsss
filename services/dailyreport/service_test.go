// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/setting"
)

func TestExistingOrganizationUsesReportCutoff(t *testing.T) {
	previous := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	t.Cleanup(func() { setting.AppDataPath = previous })

	old := dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, Accounts: map[string]string{"alice": "person"}}
	path := orgPath(7, "config.json")
	if err := dailyreport.SaveConfig(path, old); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOrgConfig(7)
	if err != nil || got.Schedule.Time != "18:30" || got.Accounts["alice"] != "person" {
		t.Fatalf("upgraded configuration: %#v, %v", got.Schedule, err)
	}
	stored, err := dailyreport.LoadConfig(path)
	if err != nil || stored.Schedule.Time != "21:00" {
		t.Fatal("reading the old configuration must not rewrite it")
	}
	if err := SaveOrgConfig(7, old); err != nil {
		t.Fatal(err)
	}
	stored, err = dailyreport.LoadConfig(path)
	if err != nil || stored.Schedule.Time != "18:30" || stored.Accounts["alice"] != "person" {
		t.Fatal("saving must retain settings and fix the report time")
	}
	loc := time.FixedZone("CST", 8*60*60)
	got.Schedule.Enabled = true
	for _, tc := range []struct {
		hour, minute int
		want         bool
	}{{18, 29, false}, {18, 30, true}, {21, 0, false}} {
		due, err := got.Schedule.Due(time.Date(2026, 11, 10, tc.hour, tc.minute, 0, 0, loc))
		if err != nil || due != tc.want {
			t.Fatalf("%02d:%02d due=%v err=%v", tc.hour, tc.minute, due, err)
		}
	}
}

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
