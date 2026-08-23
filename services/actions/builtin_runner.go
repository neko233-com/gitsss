// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/runner/pkg/builtin"

	gouuid "github.com/google/uuid"
)

const (
	builtinRunnerName      = "gitsss-builtin"
	builtinRunnerDirectory = "actions/builtin-runner"
)

var builtinRunnerManager struct {
	sync.Mutex
	parentCtx context.Context
	cancel    context.CancelFunc
	handle    *builtin.Handle
}

// StartBuiltinRunner starts the server-owned Actions runner after routes are mounted.
// It is safe to call before the HTTP listener is ready; the runner retries declaration.
func StartBuiltinRunner(ctx context.Context) error {
	if !setting.Actions.Enabled || !setting.Actions.BuiltinRunnerEnabled {
		return nil
	}

	builtinRunnerManager.Lock()
	defer builtinRunnerManager.Unlock()
	if builtinRunnerManager.handle != nil {
		return nil
	}

	return startBuiltinRunnerLocked(ctx)
}

func startBuiltinRunnerLocked(ctx context.Context) error {
	registration, err := createBuiltinRunnerRegistration(ctx)
	if err != nil {
		return fmt.Errorf("create built-in runner registration: %w", err)
	}

	baseDir := filepath.Join(setting.AppDataPath, builtinRunnerDirectory)
	runnerCtx, cancel := context.WithCancel(ctx)
	localURL := builtinRunnerURL()
	handle, err := builtin.Start(runnerCtx, builtin.Options{
		Address:        localURL,
		JobInstanceURL: localURL,
		WorkdirParent:  filepath.Join(baseDir, "workspaces"),
		CacheDir:       filepath.Join(baseDir, "cache"),
		CacheHost:      "127.0.0.1",
		Capacity:       setting.Actions.BuiltinRunnerCapacity,
		Labels:         setting.Actions.BuiltinRunnerLabels,
		Registration: builtin.Registration{
			ID:        registration.ID,
			UUID:      registration.UUID,
			Name:      registration.Name,
			Version:   registration.Version,
			Token:     registration.Token,
			Labels:    registration.AgentLabels,
			Ephemeral: registration.Ephemeral,
		},
	})
	if err != nil {
		cancel()
		return err
	}

	builtinRunnerManager.parentCtx = ctx
	builtinRunnerManager.cancel = cancel
	builtinRunnerManager.handle = handle
	go clearBuiltinRunnerWhenStopped(runnerCtx, handle)

	log.Info("built-in Actions runner %q started", registration.Name)
	return nil
}

func builtinRunnerURL() string {
	// A server bound to 0.0.0.0 listens on IPv4 only. Go may resolve
	// localhost to ::1 first inside a container, so keep the in-process
	// runner on an explicit IPv4 loopback address.
	return normalizeBuiltinRunnerURL(setting.LocalURL)
}

func normalizeBuiltinRunnerURL(localURL string) string {
	return strings.Replace(localURL, "://localhost", "://127.0.0.1", 1)
}

func clearBuiltinRunnerWhenStopped(ctx context.Context, handle *builtin.Handle) {
	<-ctx.Done()
	handle.Close()

	builtinRunnerManager.Lock()
	if builtinRunnerManager.handle == handle {
		builtinRunnerManager.handle = nil
		builtinRunnerManager.cancel = nil
	}
	builtinRunnerManager.Unlock()
}

// StopBuiltinRunner stops the server-owned runner without changing its config.
func StopBuiltinRunner(ctx context.Context) error {
	builtinRunnerManager.Lock()
	cancel := builtinRunnerManager.cancel
	handle := builtinRunnerManager.handle
	builtinRunnerManager.cancel = nil
	builtinRunnerManager.handle = nil
	builtinRunnerManager.Unlock()

	if cancel == nil || handle == nil {
		return nil
	}
	cancel()
	handle.Close()
	return nil
}

// RestartBuiltinRunner replaces the server-owned runner registration and poller.
func RestartBuiltinRunner(ctx context.Context) error {
	builtinRunnerManager.Lock()
	parentCtx := builtinRunnerManager.parentCtx
	builtinRunnerManager.Unlock()
	if parentCtx == nil {
		parentCtx = ctx
	}
	if err := StopBuiltinRunner(ctx); err != nil {
		return err
	}
	return StartBuiltinRunner(parentCtx)
}

// BuiltinRunnerRunning reports whether the in-process runner poller is active.
func BuiltinRunnerRunning() bool {
	builtinRunnerManager.Lock()
	defer builtinRunnerManager.Unlock()
	return builtinRunnerManager.handle != nil
}

func createBuiltinRunnerRegistration(ctx context.Context) (*actions_model.ActionRunner, error) {
	// The name is reserved for the service-owned runner. Removing only this exact
	// name prevents stale rows after a restart without touching user runners.
	if _, err := db.GetEngine(ctx).
		Where("owner_id = ? AND repo_id = ? AND name = ?", 0, 0, builtinRunnerName).
		Delete(&actions_model.ActionRunner{}); err != nil {
		return nil, err
	}

	runner := &actions_model.ActionRunner{
		UUID:        gouuid.New().String(),
		Name:        builtinRunnerName,
		Version:     setting.AppVer,
		AgentLabels: append([]string(nil), setting.Actions.BuiltinRunnerLabels...),
		Description: "Managed by the Gitsss server process",
	}
	if err := runner.GenerateToken(); err != nil {
		return nil, err
	}
	if err := actions_model.CreateRunner(ctx, runner); err != nil {
		return nil, err
	}
	return runner, nil
}

// BuiltinRunnerName returns the reserved runner name for admin UI and diagnostics.
func BuiltinRunnerName() string {
	return builtinRunnerName
}
