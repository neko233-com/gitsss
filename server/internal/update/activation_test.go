// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/launcher"
)

type fakeActivationHost struct {
	restarts     []uint64
	healthChecks []uint64
	restart      func(launcher.State) error
	healthy      func(launcher.State) error
}

func (host *fakeActivationHost) Restart(_ context.Context, state launcher.State) error {
	host.restarts = append(host.restarts, state.Generation)
	if host.restart != nil {
		return host.restart(state)
	}
	return nil
}

func (host *fakeActivationHost) Healthy(_ context.Context, state launcher.State) error {
	host.healthChecks = append(host.healthChecks, state.Generation)
	if host.healthy != nil {
		return host.healthy(state)
	}
	return nil
}

func TestActivatePublishesAndVerifiesNewGeneration(t *testing.T) {
	stageDirectory, stateDirectory, _ := prepareActivationTest(t)
	host := &fakeActivationHost{}
	result, err := Activate(context.Background(), stageDirectory, stateDirectory, host)
	if err != nil {
		t.Fatal(err)
	}
	if result.ActiveGeneration != 2 || result.RolledBack || len(host.restarts) != 1 || host.restarts[0] != 2 || len(host.healthChecks) != 1 || host.healthChecks[0] != 2 {
		t.Fatalf("result = %#v, host = %#v", result, host)
	}
	active, _, err := launcher.LoadLatest(stateDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if active.Generation != 2 || active.Directory != stageDirectory || active.Version != "1.2.3" {
		t.Fatalf("active = %#v", active)
	}
}

func TestActivateRollsBackWhenNewGenerationIsUnhealthy(t *testing.T) {
	stageDirectory, stateDirectory, oldDirectory := prepareActivationTest(t)
	host := &fakeActivationHost{healthy: func(state launcher.State) error {
		if state.Generation == 2 {
			return errors.New("new Agent unavailable")
		}
		return nil
	}}
	result, err := Activate(context.Background(), stageDirectory, stateDirectory, host)
	if !errors.Is(err, ErrActivationFailed) || !result.RolledBack || result.RollbackGeneration != 3 {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if len(host.restarts) != 2 || host.restarts[0] != 2 || host.restarts[1] != 3 || len(host.healthChecks) != 2 {
		t.Fatalf("host = %#v", host)
	}
	active, _, loadErr := launcher.LoadLatest(stateDirectory)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if active.Generation != 3 || active.Directory != oldDirectory || active.Version != "1.0.0" {
		t.Fatalf("rollback active = %#v", active)
	}
}

func TestActivateReportsRollbackRestartFailure(t *testing.T) {
	stageDirectory, stateDirectory, _ := prepareActivationTest(t)
	host := &fakeActivationHost{restart: func(state launcher.State) error {
		if state.Generation == 2 {
			return errors.New("new restart failed")
		}
		return errors.New("old restart failed")
	}}
	result, err := Activate(context.Background(), stageDirectory, stateDirectory, host)
	if !result.RolledBack || !errors.Is(err, ErrActivationFailed) || !stringsContainAll(err.Error(), "new restart failed", "old restart failed") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestActivateRejectsConcurrentOrAbandonedLock(t *testing.T) {
	stageDirectory, stateDirectory, _ := prepareActivationTest(t)
	if err := os.WriteFile(filepath.Join(stateDirectory, activationLockName), []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Activate(context.Background(), stageDirectory, stateDirectory, &fakeActivationHost{})
	if !errors.Is(err, ErrActivationFailed) || result.ActiveGeneration != 0 {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	active, _, loadErr := launcher.LoadLatest(stateDirectory)
	if loadErr != nil || active.Generation != 1 {
		t.Fatalf("active = %#v, err = %v", active, loadErr)
	}
}

func prepareActivationTest(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	oldDirectory := filepath.Join(root, "old")
	stageDirectory := filepath.Join(root, "stage")
	workingDirectory := filepath.Join(root, "work")
	for _, directory := range []string{stateDirectory, oldDirectory, stageDirectory, workingDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	executable := "gitsss-agent"
	archive := "gitsss-1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	if runtime.GOOS == "windows" {
		executable += ".exe"
		archive = "gitsss-1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip"
	}
	for _, directory := range []string{oldDirectory, stageDirectory} {
		if err := os.WriteFile(filepath.Join(directory, executable), []byte("test executable"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	previous := launcher.State{
		Format: launcher.Format, Generation: 1, Version: "1.0.0", Commit: "0123456789abcdef0123456789abcdef01234567",
		Platform: runtime.GOOS, Architecture: runtime.GOARCH, Directory: oldDirectory,
		Executable: executable, WorkingDirectory: workingDirectory,
		Arguments: []string{"work", "--server", "http://127.0.0.1:3000", "--id", "agent-1", "--token-file", filepath.Join(root, "agent.token")},
	}
	if _, err := launcher.Publish(stateDirectory, previous); err != nil {
		t.Fatal(err)
	}
	staged := StageResult{
		Version: "1.2.3", Commit: "89abcdef0123456789abcdef0123456789abcdef", PublishedAt: "2026-07-14T12:00:00Z",
		KeyID:    "0123456789abcdef",
		Artifact: Artifact{Name: archive, OS: runtime.GOOS, Arch: runtime.GOARCH, Size: 123, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Target:   stageDirectory, Files: 2, Bytes: 456, StagedAt: "2026-07-14T12:01:00Z",
	}
	metadata, err := json.Marshal(staged)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDirectory, stageReadyName), append(metadata, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	build, err := json.Marshal(map[string]string{"version": staged.Version, "commit": staged.Commit, "goos": runtime.GOOS, "goarch": runtime.GOARCH})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDirectory, "BUILDINFO.json"), append(build, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return stageDirectory, stateDirectory, oldDirectory
}

func stringsContainAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
