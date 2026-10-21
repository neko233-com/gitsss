// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hookrelay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dispatch avoids a shell only when both the dispatcher and its sole delegate are managed.
func Dispatch(ctx context.Context, mode string, args []string, input io.Reader, output, errorOutput io.Writer, getenv func(string) string) int {
	directory := getenv(EnvPrefix + "ORIGINAL_DIR")
	key := strings.ToUpper(strings.ReplaceAll(mode, "-", "_"))
	top, err := os.ReadFile(filepath.Join(directory, mode))
	digest := sha256.Sum256(top)
	managed := err == nil && hex.EncodeToString(digest[:]) == getenv(EnvPrefix+"DISPATCH_SHA_"+key) && DelegateMatches(mode, getenv)
	entries, err := os.ReadDir(filepath.Join(directory, mode+".d"))
	if err != nil {
		managed = false
	}
	for _, entry := range entries {
		if entry.Name() == "gitea" {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, mode+".d", entry.Name()))
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			managed = false
			break
		}
	}
	if managed {
		return Run(ctx, mode, args, input, output, errorOutput, getenv)
	}
	return RunCustomHook(ctx, filepath.Join(directory, mode), args, input, output, errorOutput)
}

// RunCustomHook preserves the original hook while removing the relay capability.
func RunCustomHook(ctx context.Context, path string, args []string, input io.Reader, output, errorOutput io.Writer) int {
	command := exec.CommandContext(ctx, path, args...)
	command.Stdin, command.Stdout, command.Stderr = input, output, errorOutput
	command.Env = PublicEnvironment(os.Environ())
	if err := command.Run(); err != nil {
		if code := command.ProcessState; code != nil && code.ExitCode() > 0 {
			return code.ExitCode()
		}
		return 1
	}
	return 0
}
