// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildImportPlanClassifiesTargets(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "restore")
	if err := os.MkdirAll(filepath.Join(target, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "data", "repositories"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(target, "config", "app.ini"), "same")
	writeTestFile(t, filepath.Join(target, "data", "repositories", "world.git"), "other")
	manifest := importTestManifest([]Entry{
		importTestEntry("config/app.ini", "config", "same"),
		importTestEntry("data/lfs/new-object", "lfs", "new"),
		importTestEntry("data/repositories/world.git", "repositories", "world"),
	})
	generatedAt := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
	plan, err := BuildImportPlan(Verification{Manifest: manifest}, target, generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if plan.GeneratedAt != generatedAt || plan.Summary != (ImportSummary{New: 1, Identical: 1, Conflicts: 1}) {
		t.Fatalf("unexpected plan summary: %#v", plan)
	}
	statuses := map[string]ImportStatus{}
	for _, item := range plan.Items {
		statuses[item.Path] = item.Status
		if !strings.HasPrefix(item.Destination, plan.Target+string(filepath.Separator)) {
			t.Fatalf("destination escaped target: %s", item.Destination)
		}
	}
	if statuses["config/app.ini"] != ImportIdentical || statuses["data/lfs/new-object"] != ImportNew || statuses["data/repositories/world.git"] != ImportConflict {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestBuildImportPlanMarksMissingRootAsNew(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing")
	manifest := importTestManifest([]Entry{importTestEntry("config/app.ini", "config", "config")})
	plan, err := BuildImportPlan(Verification{Manifest: manifest}, target, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.New != 1 || plan.Items[0].Status != ImportNew {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestBuildImportPlanBlocksUnsafeTargetComponents(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "restore")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(target, "data"), "not-a-directory")
	manifest := importTestManifest([]Entry{importTestEntry("data/lfs/object", "lfs", "object")})
	plan, err := BuildImportPlan(Verification{Manifest: manifest}, target, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Blocked != 1 || plan.Items[0].Status != ImportBlocked {
		t.Fatalf("unexpected blocked plan: %#v", plan)
	}

	outside := filepath.Join(directory, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	symlinkTarget := filepath.Join(directory, "symlink-restore")
	if err := os.Symlink(outside, symlinkTarget); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	plan, err = BuildImportPlan(Verification{Manifest: manifest}, symlinkTarget, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Blocked != 1 || !strings.Contains(plan.Items[0].Reason, "symlink") {
		t.Fatalf("symlink was not blocked: %#v", plan)
	}
	plan, err = BuildImportPlan(Verification{Manifest: manifest}, filepath.Join(symlinkTarget, "missing"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Blocked != 1 || !strings.Contains(plan.Items[0].Reason, "symlink") {
		t.Fatalf("ancestor symlink was not blocked: %#v", plan)
	}
}

func importTestManifest(entries []Entry) Manifest {
	return Manifest{
		Version: 1, CreatedAt: time.Now(),
		Source: Source{
			AssessmentID: "assessment-0123456789abcdef",
			ConfigDigest: "sha256:" + strings.Repeat("a", 64),
		},
		Entries: entries,
	}
}

func importTestEntry(path, kind, content string) Entry {
	digest := sha256.Sum256([]byte(content))
	return Entry{Path: path, Kind: kind, Size: int64(len(content)), SHA256: fmt.Sprintf("%x", digest)}
}
