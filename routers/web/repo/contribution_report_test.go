// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContributionReportRangeWithinLimit(t *testing.T) {
	from := time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, contributionReportRangeWithinLimit(from, from.AddDate(10, 0, 0)))
	assert.False(t, contributionReportRangeWithinLimit(from, from.AddDate(10, 0, 1)))
	assert.False(t, contributionReportRangeWithinLimit(from, from))
	assert.False(t, contributionReportRangeWithinLimit(from, from.Add(-time.Second)))
}
