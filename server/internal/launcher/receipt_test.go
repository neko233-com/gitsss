// Copyright 2026 gitsss Authors. All rights reserved.

package launcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartReceiptProvesGenerationStartedAfterThreshold(t *testing.T) {
	directory := t.TempDir()
	threshold := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	filePath, err := PublishStarted(directory, 2, 1234, threshold.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	started, err := StartedAfter(directory, 2, threshold)
	if err != nil || !started {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	if started, err := StartedAfter(directory, 1, threshold); err != nil || started {
		t.Fatalf("wrong generation started = %v, err = %v", started, err)
	}
	if err := os.WriteFile(filepath.Join(directory, filepath.Base(filePath)), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StartedAfter(directory, 2, threshold); err == nil {
		t.Fatal("tampered receipt was accepted")
	}
}
