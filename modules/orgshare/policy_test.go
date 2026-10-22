// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckQuota(t *testing.T) {
	require.NoError(t, CheckQuota(QuotaInput{Used: 10, Incoming: 5, MaxBytes: 20, MaxFile: 8}))
	require.Error(t, CheckQuota(QuotaInput{Used: 10, Incoming: 9, MaxBytes: 20, MaxFile: 8}))
	require.Error(t, CheckQuota(QuotaInput{Used: 10, Incoming: 11, MaxBytes: 20, MaxFile: 30}))
}

func TestParseContentRange(t *testing.T) {
	got, err := ParseContentRange("bytes 0-99/1000")
	require.NoError(t, err)
	assert.Equal(t, ContentRange{Start: 0, End: 99, Total: 1000}, got)
	assert.Equal(t, int64(100), got.Length())
	_, err = ParseContentRange("bytes 10-5/20")
	require.Error(t, err)
	_, err = ParseContentRange("bytes 0-20/20")
	require.Error(t, err)
}

func TestSafeParts(t *testing.T) {
	parts, err := SafeParts(`notes/../secret`)
	require.Error(t, err)
	assert.Nil(t, parts)
	parts, err = SafeParts(`design/ui/mock.png`)
	require.NoError(t, err)
	assert.Equal(t, []string{"design", "ui", "mock.png"}, parts)
	assert.Equal(t, "design/ui/mock.png", CleanJoin(parts))
}

func TestExpiryAndAccess(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	assert.True(t, Expiry(now, 0).IsZero())
	assert.Equal(t, now.AddDate(0, 0, 30), Deadline(now, DefaultExpireDays, false))
	assert.True(t, Deadline(now, DefaultExpireDays, true).IsZero())
	assert.True(t, Expired(now.Add(-time.Hour), now))
	assert.False(t, Expired(time.Time{}, now))
	assert.True(t, CanRead(true))
	assert.False(t, CanWrite(false))
	assert.True(t, CanDelete(false, true))
	assert.False(t, CanDelete(false, false))
}
