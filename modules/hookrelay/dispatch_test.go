// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hookrelay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func directFixture(t *testing.T) map[string]string {
	t.Helper()
	env := fixture(t, "update")
	top := []byte("#!/bin/sh\n[ -z \"${GITSSS_HOOK_TOKEN:-}\" ] || exit 91\ncat\nprintf '%s\\n' \"$1\"\nexit 23\n")
	if err := os.WriteFile(filepath.Join(env[EnvPrefix+"ORIGINAL_DIR"], "update"), top, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(top)
	env[EnvPrefix+"DISPATCH_SHA_UPDATE"] = hex.EncodeToString(digest[:])
	return env
}

func TestDirectManagedUpdateKeepsPullGuard(t *testing.T) {
	env := directFixture(t)
	for _, entry := range []struct {
		ref  string
		code int
	}{{"refs/heads/main", 0}, {"refs/pull/1/head", 1}} {
		if code := Dispatch(t.Context(), "update", []string{entry.ref, "old", "new"}, strings.NewReader(""), io.Discard, io.Discard, func(key string) string { return env[key] }); code != entry.code {
			t.Fatalf("direct update changed ref protection: %s returned %d", entry.ref, code)
		}
	}
}

func TestDirectCustomAndChangedDispatcherFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows retains the shell proxy and does not install direct hook links")
	}
	for _, changed := range []bool{false, true} {
		env := directFixture(t)
		directory := env[EnvPrefix+"ORIGINAL_DIR"]
		if changed {
			env[EnvPrefix+"DISPATCH_SHA_UPDATE"] = "different dispatcher"
		} else if err := os.WriteFile(filepath.Join(directory, "update.d", "custom"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(EnvPrefix+"TOKEN", "fixture-must-not-reach-custom-script")
		var output, errorOutput bytes.Buffer
		code := Dispatch(context.Background(), "update", []string{"refs/heads/main", "old", "new"}, strings.NewReader("input\n"), &output, &errorOutput, func(key string) string { return env[key] })
		if code != 23 || output.String() != "input\nrefs/heads/main\n" || errorOutput.Len() != 0 {
			t.Fatalf("fallback lost the original input, arguments, exit status or credential isolation: %d", code)
		}
	}
}

func TestPublicEnvironmentRestoresNestedGitHooks(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git is not installed")
	}
	original := filepath.Join(t.TempDir(), "hooks with ' quote")
	config := filepath.Join(t.TempDir(), "empty.gitconfig")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), EnvPrefix+"ORIGINAL_DIR="+original,
		"GIT_CONFIG_PARAMETERS='core.hooksPath=/temporary-relay' 'credential.fixture=retained'",
		"GIT_CONFIG_GLOBAL="+config, "GIT_CONFIG_NOSYSTEM=1")
	for key, expected := range map[string]string{"core.hooksPath": original, "credential.fixture": "retained"} {
		command := exec.CommandContext(t.Context(), gitPath, "config", "--get", key)
		command.Env = PublicEnvironment(environment)
		output, err := command.Output()
		if err != nil || strings.TrimSpace(string(output)) != expected {
			t.Fatalf("nested Git lost %s: %q %v", key, output, err)
		}
	}
}
