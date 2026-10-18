// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import "testing"

func TestSoftwareManagedDownloadReceipt(t *testing.T) {
	for _, tc := range []struct {
		transfer string
		state    string
		want     bool
	}{
		{"", "paused", true},
		{"", "failed", true},
		{"", "complete", true},
		{"managed", "paused", false},
		{"managed", "failed", false},
		{"managed", "complete", true},
	} {
		if got := softwareDownloadReceiptRequired(tc.transfer, tc.state); got != tc.want {
			t.Errorf("receipt(%q, %q) = %v, want %v", tc.transfer, tc.state, got, tc.want)
		}
	}
}
