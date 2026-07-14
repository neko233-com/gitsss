// Copyright 2026 gitsss Authors. All rights reserved.

package autostart

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeTransactionHost struct {
	events      []string
	files       map[string][]byte
	failCommand string
}

func newFakeTransactionHost() *fakeTransactionHost {
	return &fakeTransactionHost{files: make(map[string][]byte)}
}

func (host *fakeTransactionHost) EnsureDirectory(path string, _ fs.FileMode) (bool, error) {
	host.events = append(host.events, "mkdir "+path)
	return true, nil
}

func (host *fakeTransactionHost) CreateToken(_ context.Context, _ Platform, path string, content []byte) error {
	host.events = append(host.events, "token "+path)
	host.files[path] = append([]byte(nil), content...)
	return nil
}

func (host *fakeTransactionHost) CreateDefinition(path string, content []byte, _ fs.FileMode) error {
	host.events = append(host.events, "definition "+path)
	host.files[path] = append([]byte(nil), content...)
	return nil
}

func (host *fakeTransactionHost) Run(_ context.Context, command Command) error {
	host.events = append(host.events, "run "+command.Executable)
	if command.Executable == host.failCommand {
		return errors.New("injected command failure")
	}
	return nil
}

func (host *fakeTransactionHost) RemoveFile(path string) error {
	host.events = append(host.events, "remove-file "+path)
	delete(host.files, path)
	return nil
}

func (host *fakeTransactionHost) RemoveDirectory(path string) error {
	host.events = append(host.events, "remove-directory "+path)
	return nil
}

func TestInstallCreatesProtectedInputsBeforeStartingService(t *testing.T) {
	plan := transactionTestPlan(t)
	host := newFakeTransactionHost()
	if err := installWithHost(context.Background(), plan, "work-token", host); err != nil {
		t.Fatal(err)
	}
	if got := string(host.files[plan.TokenFile]); got != "work-token\n" {
		t.Fatalf("token file = %q", got)
	}
	if got := string(host.files[plan.DefinitionPath]); got != plan.Definition {
		t.Fatalf("definition = %q", got)
	}
	events := strings.Join(host.events, "\n")
	for _, sequence := range []string{"token " + plan.TokenFile, "definition " + plan.DefinitionPath, "run install-service"} {
		if !strings.Contains(events, sequence) {
			t.Fatalf("events do not contain %q:\n%s", sequence, events)
		}
	}
	if strings.Index(events, "token ") > strings.Index(events, "run install-service") || strings.Index(events, "definition ") > strings.Index(events, "run install-service") {
		t.Fatalf("service started before inputs were published:\n%s", events)
	}
}

func TestInstallRollsBackServiceAndFilesAfterCommandFailure(t *testing.T) {
	plan := transactionTestPlan(t)
	host := newFakeTransactionHost()
	host.failCommand = "install-service"
	if err := installWithHost(context.Background(), plan, "work-token", host); err == nil {
		t.Fatal("installation failure was not returned")
	}
	if len(host.files) != 0 {
		t.Fatalf("rollback left files: %#v", host.files)
	}
	events := strings.Join(host.events, "\n")
	for _, expected := range []string{"run uninstall-service", "remove-file " + plan.DefinitionPath, "remove-file " + plan.TokenFile} {
		if !strings.Contains(events, expected) {
			t.Fatalf("rollback did not contain %q:\n%s", expected, events)
		}
	}
}

func TestUninstallRetainsCredentialsWhenServiceRemovalFails(t *testing.T) {
	plan := transactionTestPlan(t)
	host := newFakeTransactionHost()
	host.files[plan.TokenFile] = []byte("work-token\n")
	host.files[plan.DefinitionPath] = []byte(plan.Definition)
	host.failCommand = "uninstall-service"
	if err := uninstallWithHost(context.Background(), plan, host); err == nil {
		t.Fatal("uninstall failure was not returned")
	}
	if len(host.files) != 2 {
		t.Fatalf("failed uninstall removed recovery files: %#v", host.files)
	}
}

func TestInstallRejectsMultilineTokenBeforeHostChanges(t *testing.T) {
	host := newFakeTransactionHost()
	if err := installWithHost(context.Background(), transactionTestPlan(t), "first\nsecond", host); err == nil {
		t.Fatal("multiline token was accepted")
	}
	if len(host.events) != 0 {
		t.Fatalf("invalid token changed host: %#v", host.events)
	}
}

func TestAtomicCreateNoReplacePreservesExistingFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "agent.token")
	if err := atomicCreateNoReplace(filePath, []byte("first"), 0o600, nil); err != nil {
		t.Fatal(err)
	}
	if err := atomicCreateNoReplace(filePath, []byte("second"), 0o600, nil); err == nil {
		t.Fatal("existing file was replaced")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "first" {
		t.Fatalf("content = %q", content)
	}
	partials, err := filepath.Glob(filepath.Join(filepath.Dir(filePath), ".gitsss-partial-*"))
	if err != nil || len(partials) != 0 {
		t.Fatalf("partial files = %#v, %v", partials, err)
	}
}

func transactionTestPlan(t *testing.T) Plan {
	t.Helper()
	root := t.TempDir()
	return Plan{
		Platform: Platform(runtime.GOOS), DefinitionPath: filepath.Join(root, "service.definition"),
		TokenFile: filepath.Join(root, "agent.token"), Directories: []string{filepath.Join(root, "work")},
		Definition: "service definition\n",
		Install:    []Command{{Executable: "install-service"}},
		Uninstall:  []Command{{Executable: "uninstall-service"}},
	}
}
