// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmitQuotaAndDisk(t *testing.T) {
	err := Admit(AdmitInput{
		Used: 90 << 30, Incoming: 20 << 30, MaxBytes: DefaultMaxBytes, MaxFile: DefaultMaxFileBytes,
		Available: 200 << 30, Reserve: DefaultReserveBytes, DiskKnown: true,
	})
	var limit *LimitError
	require.ErrorAs(t, err, &limit)
	assert.Equal(t, "quota", limit.Code)

	err = Admit(AdmitInput{
		Used: 0, Incoming: 8 << 30, MaxBytes: DefaultMaxBytes, MaxFile: DefaultMaxFileBytes,
		Available: 8 << 30, Reserve: DefaultReserveBytes, DiskKnown: true,
	})
	require.ErrorAs(t, err, &limit)
	assert.Equal(t, "disk", limit.Code)
	assert.Contains(t, err.Error(), "磁盘空间不足")

	require.NoError(t, Admit(AdmitInput{
		Used: 0, Incoming: 8 << 30, MaxBytes: DefaultMaxBytes, MaxFile: DefaultMaxFileBytes,
		Available: 20 << 30, Reserve: DefaultReserveBytes, DiskKnown: true,
	}))

	err = Admit(AdmitInput{Incoming: 1, DiskKnown: false})
	require.ErrorAs(t, err, &limit)
	assert.Equal(t, "disk_unknown", limit.Code)
}
