// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "github.com/neko233-com/gitsss/models/actions"
)

func TestSortDashboardWorkerItemsRunningFirst(t *testing.T) {
	items := []*DashboardWorkerItem{
		{Status: actions_model.StatusWaiting, Updated: 30},
		{Status: actions_model.StatusRunning, IsRunning: true, Updated: 1},
		{Status: actions_model.StatusRunning, IsRunning: true, Updated: 20},
	}

	sortDashboardWorkerItems(items)
	if !items[0].IsRunning || !items[1].IsRunning {
		t.Fatalf("running jobs must be listed first: %#v", items)
	}
	if items[0].Updated != 20 || items[1].Updated != 1 {
		t.Fatalf("running jobs must be newest first: %#v", items)
	}
	if items[2].IsRunning {
		t.Fatalf("queued job must follow running jobs: %#v", items)
	}
}

func TestWorkerSearchTextHandlesMissingJob(t *testing.T) {
	item := &DashboardWorkerItem{Status: actions_model.StatusWaiting}
	if got := workerSearchText(item); got != "waiting" {
		t.Fatalf("unexpected search text: %q", got)
	}
}
