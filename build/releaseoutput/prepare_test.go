// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrepareOutputRemovesArtifactsFromPriorVersions(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist", "release")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{
		"gitsss-1.18.0-linux-amd64",
		"gitsss-1.18.0-linux-amd64.xz",
		"gitsss-1.18.0-linux-amd64.xz.sha256",
		"gitsss-1.18.0-windows-amd64.tar.gz",
		"gitsss-src-1.18.0.tar.gz.sha256",
		"SHA256SUMS",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(output, name), []byte("generated"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	count, target, err := prepareOutput(root, output, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(names) || target != output {
		t.Fatalf("prepareOutput() = (%d, %q), want (%d, %q)", count, target, len(names), output)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("output still contains %d entries", len(entries))
	}
}

func TestPrepareOutputCheckOnlyDoesNotCreateOrRemove(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist", "release")
	if count, target, err := prepareOutput(root, output, true); err != nil || count != 0 || target != output {
		t.Fatalf("prepareOutput() = (%d, %q, %v)", count, target, err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("check-only created output directory: %v", err)
	}

	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(output, "gitsss-1.18.0-linux-amd64")
	if err := os.WriteFile(artifact, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if count, _, err := prepareOutput(root, output, true); err != nil || count != 1 {
		t.Fatalf("check-only prepareOutput() = (%d, %v)", count, err)
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Fatalf("check-only removed prior artifact: %v", err)
	}
}

func TestPrepareOutputRefusesUnexpectedContentWithoutDeleting(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist", "release")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(output, "gitsss-1.18.0-linux-amd64")
	userFile := filepath.Join(output, "notes.txt")
	if err := os.WriteFile(artifact, []byte("package"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareOutput(root, output, false); err == nil {
		t.Fatal("prepareOutput() unexpectedly accepted a user file")
	}
	for _, path := range []string{artifact, userFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("prepareOutput() removed %s after refusal: %v", path, err)
		}
	}
}

func TestPrepareOutputNeverRecursivelyRemovesDirectories(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "dist", "release")
	nested := filepath.Join(output, "user-data")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(nested, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareOutput(root, output, false); err == nil {
		t.Fatal("prepareOutput() unexpectedly accepted a nested directory")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("prepareOutput() removed nested user data: %v", err)
	}
}

func TestPrepareOutputRejectsRepositoryRootAndExternalPath(t *testing.T) {
	root := t.TempDir()
	if _, _, err := prepareOutput(root, root, false); err == nil {
		t.Fatal("prepareOutput() accepted repository root")
	}
	external := t.TempDir()
	if _, _, err := prepareOutput(root, external, false); err == nil {
		t.Fatal("prepareOutput() accepted an output outside the repository")
	}
}

func TestPrepareOutputRejectsSymlinkedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating directory symlinks may require Windows developer mode")
	}
	root := t.TempDir()
	external := t.TempDir()
	link := filepath.Join(root, "dist")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("cannot create symlink in test environment: %v", err)
	}
	if _, _, err := prepareOutput(root, filepath.Join("dist", "release"), false); err == nil {
		t.Fatal("prepareOutput() accepted a symlinked output path")
	}
}

func TestIsReleaseArtifact(t *testing.T) {
	for _, name := range []string{
		"gitsss-1.19.4-darwin-arm64.tar.gz",
		"gitsss-1.19.4-gogit-windows-386.exe.xz",
		"gitsss-hook-1.19.4-linux-amd64.xz.sha256",
		"gitsss-src-1.19.4.tar.gz",
		"SHA256SUMS",
	} {
		if !isReleaseArtifact(name) {
			t.Errorf("isReleaseArtifact(%q) = false", name)
		}
	}
	for _, name := range []string{"gitsss-notes.txt", "README.md", "gitsss-1.19.4-wasm-wasm.tar.gz"} {
		if isReleaseArtifact(name) {
			t.Errorf("isReleaseArtifact(%q) = true", name)
		}
	}
}
