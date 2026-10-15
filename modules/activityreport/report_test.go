// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activityreport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeUnix(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 10, 15, 16, 0, 0, 0, loc)
	today := time.Date(2026, 10, 15, 9, 0, 0, 0, loc).Unix()
	lastMonth := time.Date(2026, 9, 2, 9, 0, 0, 0, loc).Unix()
	summary := SummarizeUnix([]int64{today, today, lastMonth}, now)
	assert.Len(t, summary.Daily, 14)
	assert.Equal(t, "2026-10-15", summary.Daily[0].Label)
	assert.Equal(t, 2, summary.Daily[0].Commits)
	assert.Len(t, summary.Monthly, 12)
	assert.Equal(t, "2026-10", summary.Monthly[0].Label)
	assert.Equal(t, 2, summary.Monthly[0].Commits)
	assert.Equal(t, "2026-09", summary.Monthly[1].Label)
	assert.Equal(t, 1, summary.Monthly[1].Commits)
}
