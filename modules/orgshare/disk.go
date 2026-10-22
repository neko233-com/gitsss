// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

// Volume is the filesystem that holds organization share bytes.
type Volume struct {
	CapacityBytes  int64
	AvailableBytes int64
}

// MeasureVolume reads free space for path. Tests may replace it.
var MeasureVolume = measureVolume
