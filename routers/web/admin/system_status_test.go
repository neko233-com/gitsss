// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestSystemStatusMemoryAccounting(t *testing.T) {
	status := makeSystemStatus(runtime.MemStats{Alloc: 32 << 20, TotalAlloc: 5 << 30, Sys: 100 << 20, HeapAlloc: 32 << 20, HeapSys: 80 << 20, HeapReleased: 20 << 20})
	if status.HeapAllocatedBytes != 32<<20 || status.HeapCapacityBytes != 80<<20 || status.SystemBytes != 100<<20 || status.RetainedBytes != 80<<20 {
		t.Fatalf("incorrect current memory accounting: %+v", status)
	}
	if status.MemTotal == status.MemAllocated || status.MemRetained != "80 MiB" {
		t.Fatalf("cumulative and current values: %+v", status)
	}
	if status := makeSystemStatus(runtime.MemStats{Sys: 10, HeapReleased: 20}); status.RetainedBytes != 0 {
		t.Fatal("released memory must not underflow")
	}
	if status := makeSystemStatus(runtime.MemStats{}); status.HeapCapacityBytes != 0 || status.SystemBytes != 0 {
		t.Fatal("zero capacity must stay unknown")
	}
}

func TestSystemStatusConcurrentCachedReads(t *testing.T) {
	want := systemStatus{MemAllocated: "cached"}
	systemStatusCache.Lock()
	previous, expiry := systemStatusCache.value, systemStatusCache.expires
	systemStatusCache.value, systemStatusCache.expires = want, time.Now().Add(time.Hour)
	systemStatusCache.Unlock()
	t.Cleanup(func() {
		systemStatusCache.Lock()
		systemStatusCache.value, systemStatusCache.expires = previous, expiry
		systemStatusCache.Unlock()
	})
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			if got := readSystemStatus(); got != want {
				t.Errorf("cache = %+v", got)
			}
		})
	}
	group.Wait()
}
