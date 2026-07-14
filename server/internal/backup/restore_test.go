// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestRestoreIsolatedPublishesVerifiedContent(t *testing.T) {
	directory := t.TempDir()
	backupPath, assessmentID := createRestoreTestBackup(t, directory)
	target := filepath.Join(directory, "isolated")
	result, err := RestoreIsolated(backupPath, "restore test passphrase", target, assessmentID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 || result.Target != target || result.ContentPath != filepath.Join(target, "content") {
		t.Fatalf("unexpected result: %#v", result)
	}
	config, err := os.ReadFile(filepath.Join(result.ContentPath, "config", "app.ini"))
	if err != nil || string(config) != "[database]\nPASSWD = secret\n" {
		t.Fatalf("restored config = %q, %v", config, err)
	}
	repository, err := os.ReadFile(filepath.Join(result.ContentPath, "data", "repositories", "world.git"))
	if err != nil || string(repository) != "git-object-data" {
		t.Fatalf("restored repository = %q, %v", repository, err)
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".isolated.restore-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary restores remain: %v, %v", matches, err)
	}
}

func TestRestoreIsolatedRejectsConfirmationAndExistingTarget(t *testing.T) {
	directory := t.TempDir()
	backupPath, assessmentID := createRestoreTestBackup(t, directory)
	target := filepath.Join(directory, "isolated")
	if _, err := RestoreIsolated(backupPath, "restore test passphrase", target, "assessment-wrong"); !errors.Is(err, ErrConfirmation) {
		t.Fatalf("confirmation error = %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrong confirmation created target: %v", err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep")
	writeTestFile(t, sentinel, "keep")
	if _, err := RestoreIsolated(backupPath, "restore test passphrase", target, assessmentID); !errors.Is(err, ErrTargetExists) {
		t.Fatalf("existing target error = %v", err)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing target changed: %q, %v", content, err)
	}
}

func TestRestoreIsolatedExactRejectsDifferentContainer(t *testing.T) {
	directory := t.TempDir()
	backupPath, assessmentID := createRestoreTestBackup(t, directory)
	target := filepath.Join(directory, "isolated")
	if _, err := RestoreIsolatedExact(backupPath, "restore test passphrase", target, assessmentID, strings.Repeat("0", 64)); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("container confirmation error = %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected container created target: %v", err)
	}
}

func TestRestoreIsolatedRejectsInvalidBackupBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	backupPath, assessmentID := createRestoreTestBackup(t, directory)
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	content[len(content)-1] ^= 0xff
	corruptPath := filepath.Join(directory, "corrupt.gitsss-backup")
	writeTestBytes(t, corruptPath, content)
	target := filepath.Join(directory, "corrupt-target")
	if _, err := RestoreIsolated(corruptPath, "restore test passphrase", target, assessmentID); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("corrupt restore error = %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt restore created target: %v", err)
	}
}

func createRestoreTestBackup(t *testing.T, directory string) (string, string) {
	t.Helper()
	configPath := filepath.Join(directory, "source-app.ini")
	repositoryPath := filepath.Join(directory, "source-repositories")
	if err := os.Mkdir(repositoryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "[database]\nPASSWD = secret\n")
	writeTestFile(t, filepath.Join(repositoryPath, "world.git"), "git-object-data")
	assessmentID := "assessment-0123456789abcdef"
	plan, err := BuildPlan(takeover.Report{
		ID: assessmentID, ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("a", 64),
		Assets:       []takeover.Asset{{Kind: "repositories", Path: repositoryPath, Exists: true, Readable: true}},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(directory, "restore.gitsss-backup")
	if err := Export(backupPath, "restore test passphrase", plan); err != nil {
		t.Fatal(err)
	}
	return backupPath, assessmentID
}
