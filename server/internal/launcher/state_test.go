// Copyright 2026 gitsss Authors. All rights reserved.

package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublishAndLoadLatestImmutableState(t *testing.T) {
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	versionDirectory := filepath.Join(root, "versions", "1.2.3")
	if err := os.MkdirAll(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(versionDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := "gitsss-agent"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if err := os.WriteFile(filepath.Join(versionDirectory, executable), []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	state := State{
		Format: Format, Generation: 1, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		Platform: runtime.GOOS, Architecture: runtime.GOARCH,
		Directory: versionDirectory, Executable: executable, WorkingDirectory: root,
		Arguments: []string{"work", "--server", "https://gitsss.internal", "--id", "studio-agent", "--token-file", filepath.Join(root, "agent.token")},
	}
	firstPath, err := Publish(stateDirectory, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(stateDirectory, state); err == nil {
		t.Fatal("existing generation was overwritten")
	}
	state.Generation = 2
	state.Version = "1.2.4"
	if _, err := Publish(stateDirectory, state); err != nil {
		t.Fatal(err)
	}
	loaded, latestPath, err := LoadLatest(stateDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Generation != 2 || loaded.Version != "1.2.4" || latestPath == firstPath {
		t.Fatalf("loaded = %#v, %s", loaded, latestPath)
	}
	if _, err := ExecutablePath(loaded); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLatestRejectsTamperingAndSymlinkExecutable(t *testing.T) {
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	if err := os.Mkdir(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(stateDirectory, StateFilename(1))
	if err := os.WriteFile(filePath, []byte(`{"format":"gitsss-launcher-v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadLatest(stateDirectory); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("tampered state error = %v", err)
	}

	versionDirectory := filepath.Join(root, "version")
	if err := os.Mkdir(versionDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := "gitsss-agent"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(versionDirectory, executable)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	state := State{
		Format: Format, Generation: 2, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		Platform: runtime.GOOS, Architecture: runtime.GOARCH,
		Directory: versionDirectory, Executable: executable, WorkingDirectory: root, Arguments: []string{"work"},
	}
	if _, err := ExecutablePath(state); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("symlink executable error = %v", err)
	}
}
