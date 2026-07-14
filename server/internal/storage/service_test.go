// Copyright 2026 gitsss Authors. All rights reserved.

package storage

import "testing"

func TestServiceReportsUsageAndProtectsWrites(t *testing.T) {
	service := NewService(100, []Usage{{Category: CategoryGitObjects, Bytes: 40}, {Category: CategoryLFSAssets, Bytes: 47}})

	snapshot := service.Snapshot()
	if snapshot.AvailableBytes != 13 || !snapshot.Warning || snapshot.WriteProtected {
		t.Fatalf("snapshot = %#v, want warning with 13 bytes available", snapshot)
	}
	if admission := service.Admit(8); admission.Allowed || !admission.WriteProtected {
		t.Fatalf("admission = %#v, want protected write", admission)
	}
	if admission := service.Admit(14); admission.Allowed || admission.Reason != "insufficient capacity" {
		t.Fatalf("admission = %#v, want capacity rejection", admission)
	}
}
