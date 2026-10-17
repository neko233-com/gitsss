// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContributionReportNumstat(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, zone)
	to := from.AddDate(0, 1, 0)
	commit := func(name, email string, at time.Time, stats string) string {
		return fmt.Sprintf("\x00commit:abc\x00%s\x00%s\x00%d\x00\n%s", name, email, at.Unix(), stats)
	}
	input := commit("Alice", "A@example.com", from, "3\t1\tfile\twith-tab.txt\x00-\t-\tasset.bin\x00") +
		commit("Alice", "a@example.com", from.AddDate(0, 0, 1), "2\t0\trename\nnew.txt\x00") +
		commit("Outside", "x@example.com", to, "9\t0\tout.txt\x00")
	report, err := parseContributionStream(strings.NewReader(input), from, to, zone)
	require.NoError(t, err)
	assert.Equal(t, 2, report.Commits)
	assert.EqualValues(t, 5, report.Additions)
	assert.EqualValues(t, 1, report.Deletions)
	assert.Equal(t, 3, report.Files)
	assert.Equal(t, 1, report.BinaryFiles)
	require.Len(t, report.Authors, 1)
	assert.Len(t, report.Authors[0].Days, 2)
	assert.NotContains(t, fmt.Sprint(report), "example.com")
	_, err = parseContributionStream(strings.NewReader("\x00commit:a\x00Name"), from, to, zone)
	require.Error(t, err)
}
