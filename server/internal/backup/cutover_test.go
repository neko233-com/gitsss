// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCutoverAndRollbackPreserveBothGenerations(t *testing.T) {
	root := t.TempDir()
	restored := filepath.Join(root, "imports", "old-gitea")
	active := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(restored, "content", "repositories", "team.git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restored, "content", "repositories", "team.git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(active, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "old.db"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	result, err := CutoverVerified(CutoverOptions{
		AssessmentID: "assessment-1", ContainerSHA256: strings.Repeat("a", 64),
		RestoredTarget: restored, ActivePath: active, Confirmation: "CUTOVER assessment-1",
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.PreviousActive || result.RollbackPath == "" || result.CutoverAt != now {
		t.Fatalf("cutover = %#v", result)
	}
	if content, err := os.ReadFile(filepath.Join(active, "repositories", "team.git", "HEAD")); err != nil || string(content) != "ref: refs/heads/main\n" {
		t.Fatalf("active content = %q, %v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(result.RollbackPath, "old.db")); err != nil || string(content) != "old" {
		t.Fatalf("rollback content = %q, %v", content, err)
	}

	rolledBack, err := RollbackCutover(active, result.StatePath, result.ID, "ROLLBACK "+result.ID, func() time.Time { return now.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	if !rolledBack.RestoredPrevious || rolledBack.RolledBackAt != now.Add(time.Hour) {
		t.Fatalf("rollback = %#v", rolledBack)
	}
	if content, err := os.ReadFile(filepath.Join(active, "old.db")); err != nil || string(content) != "old" {
		t.Fatalf("restored previous content = %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(rolledBack.ArchivedPath, "repositories", "team.git", "HEAD")); err != nil {
		t.Fatalf("cutover generation was not archived: %v", err)
	}
}

func TestRollbackRefusesToDiscardPostCutoverWrites(t *testing.T) {
	root := t.TempDir()
	restored := filepath.Join(root, "import")
	active := filepath.Join(root, "active")
	if err := os.MkdirAll(filepath.Join(restored, "content"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restored, "content", "state.db"), []byte("verified"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CutoverVerified(CutoverOptions{
		AssessmentID: "assessment-2", ContainerSHA256: strings.Repeat("b", 64),
		RestoredTarget: restored, ActivePath: active, Confirmation: "CUTOVER assessment-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, "new-write"), []byte("must survive"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = RollbackCutover(active, result.StatePath, result.ID, "ROLLBACK "+result.ID, nil)
	if !errors.Is(err, ErrActiveChanged) {
		t.Fatalf("rollback error = %v, want active-changed refusal", err)
	}
	if content, readErr := os.ReadFile(filepath.Join(active, "new-write")); readErr != nil || string(content) != "must survive" {
		t.Fatalf("post-cutover write changed: %q, %v", content, readErr)
	}
}

func TestCutoverRequiresExactHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	restored := filepath.Join(root, "import")
	if err := os.MkdirAll(filepath.Join(restored, "content"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := CutoverVerified(CutoverOptions{
		AssessmentID: "assessment-3", ContainerSHA256: strings.Repeat("c", 64),
		RestoredTarget: restored, ActivePath: filepath.Join(root, "active"), Confirmation: "assessment-3",
	})
	if !errors.Is(err, ErrConfirmation) {
		t.Fatalf("cutover error = %v, want confirmation refusal", err)
	}
}

func TestRollbackRecoversInterruptedPreparedCutover(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "active")
	rollback := filepath.Join(root, "active.rollback-cutover-recovery")
	if err := os.MkdirAll(rollback, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rollback, "old.db"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "staged")
	if err := os.MkdirAll(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := treeDigest(staged)
	if err != nil {
		t.Fatal(err)
	}
	statePath := active + ".gitsss-cutover.json"
	marker := cutoverState{
		Version: cutoverStateVersion, ID: "cutover-recovery", AssessmentID: "assessment",
		ContainerSHA256: strings.Repeat("d", 64), ActivePath: active, RollbackPath: rollback,
		PreviousActive: true, ActivatedSHA256: digest, Status: "prepared", CutoverAt: time.Now().UTC(),
	}
	if err := writeCutoverState(statePath, marker, true); err != nil {
		t.Fatal(err)
	}
	result, err := RollbackCutover(active, statePath, marker.ID, "ROLLBACK "+marker.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestoredPrevious || result.ArchivedPath != "" {
		t.Fatalf("recovery = %#v", result)
	}
	if content, err := os.ReadFile(filepath.Join(active, "old.db")); err != nil || string(content) != "old" {
		t.Fatalf("recovered old generation = %q, %v", content, err)
	}
}
