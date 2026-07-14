// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/launcher"
)

func TestRunLauncherExecutesLatestState(t *testing.T) {
	if os.Getenv("GITSSS_LAUNCHER_HELPER") == "1" {
		os.Stdout.WriteString("helper-ready")
		return
	}
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	versionDirectory := filepath.Join(root, "version")
	if err := os.Mkdir(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(versionDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := "gitsss-agent"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(testExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDirectory, executable), content, 0o700); err != nil {
		t.Fatal(err)
	}
	state := launcher.State{
		Format: launcher.Format, Generation: 1, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		Directory: versionDirectory, Executable: executable, WorkingDirectory: root,
		Arguments: []string{"-test.run=^TestRunLauncherExecutesLatestState$"},
	}
	if _, err := launcher.Publish(stateDirectory, state); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_LAUNCHER_HELPER", "1")
	var stdout bytes.Buffer
	if err := runLauncher(context.Background(), stateDirectory, time.Second, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("helper-ready")) || !bytes.Contains(stdout.Bytes(), []byte("generation 1")) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
