// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestExportVerifyRoundTripAndRejectCorruption(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	repositoryPath := filepath.Join(directory, "repositories")
	if err := os.Mkdir(repositoryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "[database]\nPASSWD = secret\n")
	writeTestFile(t, filepath.Join(repositoryPath, "world.git"), "git-object-data")
	report := takeover.Report{
		ID: "assessment-0123456789abcdef", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("a", 64), GiteaVersion: "1.26.0",
		Assets: []takeover.Asset{{Kind: "repositories", Path: repositoryPath, Exists: true, Readable: true}},
	}
	createdAt := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	plan, err := BuildPlan(report, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Manifest.Entries) != 2 || plan.Manifest.Entries[0].Path != "config/app.ini" || plan.Manifest.Entries[1].Path != "data/repositories/world.git" {
		t.Fatalf("unexpected entries: %#v", plan.Manifest.Entries)
	}
	output := filepath.Join(directory, "takeover.gitsss-backup")
	if err := Export(output, "a sufficiently long test passphrase", plan); err != nil {
		t.Fatal(err)
	}
	verification, err := Verify(output, "a sufficiently long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	container, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(container)
	if verification.Files != 2 || verification.Manifest.Source.AssessmentID != report.ID || verification.Manifest.CreatedAt != createdAt || verification.ContainerBytes != int64(len(container)) || verification.ContainerSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("unexpected verification: %#v", verification)
	}
	if _, err := Verify(output, "the wrong passphrase"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("wrong passphrase error = %v", err)
	}

	original := container
	assertCorruptBackup(t, directory, original, "tampered", func(content []byte) []byte {
		content[len(content)-1] ^= 0xff
		return content
	})
	assertCorruptBackup(t, directory, original, "truncated", func(content []byte) []byte {
		return content[:len(content)-8]
	})
	assertCorruptBackup(t, directory, original, "trailing", func(content []byte) []byte {
		return append(content, 0x01)
	})
}

func TestExportRejectsChangedSourceAndRemovesPartialFile(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	writeTestFile(t, configPath, "original")
	plan, err := BuildPlan(takeover.Report{
		ID: "assessment-0123456789abcdef", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("b", 64),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "modified")
	output := filepath.Join(directory, "changed.gitsss-backup")
	if err := Export(output, "a sufficiently long test passphrase", plan); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("Export error = %v", err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output should not exist: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".changed.gitsss-backup.partial-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("partial files remain: %v, %v", matches, err)
	}
}

func TestBuildPlanRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	assetsPath := filepath.Join(directory, "assets")
	if err := os.Mkdir(assetsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, configPath, "config")
	target := filepath.Join(directory, "target")
	writeTestFile(t, target, "secret")
	if err := os.Symlink(target, filepath.Join(assetsPath, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := BuildPlan(takeover.Report{
		ID: "assessment-0123456789abcdef", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("c", 64),
		Assets:       []takeover.Asset{{Kind: "attachments", Path: assetsPath, Exists: true, Readable: true}},
	}, time.Now())
	if !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("BuildPlan error = %v", err)
	}
}

func TestValidateManifestRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	manifest := Manifest{
		Version: 1, CreatedAt: time.Now(),
		Source:  Source{AssessmentID: "assessment-0123456789abcdef", ConfigDigest: "sha256:" + strings.Repeat("d", 64)},
		Entries: []Entry{{Path: "../escape", Kind: "config", Size: 1, SHA256: strings.Repeat("e", 64)}},
	}
	if err := validateManifest(manifest); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("unsafe path error = %v", err)
	}
	manifest.Entries = []Entry{
		{Path: "config/app.ini", Kind: "config", Size: 1, SHA256: strings.Repeat("e", 64)},
		{Path: "config/app.ini", Kind: "config", Size: 1, SHA256: strings.Repeat("f", 64)},
	}
	if err := validateManifest(manifest); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("duplicate path error = %v", err)
	}
}

func assertCorruptBackup(t *testing.T, directory string, original []byte, name string, mutate func([]byte) []byte) {
	t.Helper()
	content := append([]byte(nil), original...)
	content = mutate(content)
	filePath := filepath.Join(directory, name+".gitsss-backup")
	writeTestBytes(t, filePath, content)
	if _, err := Verify(filePath, "a sufficiently long test passphrase"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("%s error = %v", name, err)
	}
}

func writeTestFile(t *testing.T, filePath, content string) {
	t.Helper()
	writeTestBytes(t, filePath, []byte(content))
}

func writeTestBytes(t *testing.T, filePath string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
