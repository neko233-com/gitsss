package garbage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAndCleanOnlyExpiredManagedEntries(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "old-task")
	recent := filepath.Join(root, "recent-task")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "output.bin"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recent, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	oldTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now, now); err != nil {
		t.Fatal(err)
	}
	service, err := NewService([]Root{{Kind: "runner_workdir", Path: root, MaxAge: time.Hour}})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	report, err := service.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 1 || report.Bytes != 32 || report.Files != 1 {
		t.Fatalf("report = %#v", report)
	}
	if _, _, err := service.Clean("wrong"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("invalid token error = %v", err)
	}
	bytes, files, err := service.Clean(report.Token)
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 32 || files != 1 {
		t.Fatalf("removed = %d bytes, %d files", bytes, files)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old entry still exists: %v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("recent entry removed: %v", err)
	}
}

func TestCleanRevalidatesAge(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	oldTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	service, err := NewService([]Root{{Kind: "runner_workdir", Path: root, MaxAge: time.Hour}})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	report, err := service.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Clean(report.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active entry removed: %v", err)
	}
}
