// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"path/filepath"

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

// StartBuiltinRunner starts the server-owned Actions runner after routes are mounted.
// It is safe to call before the HTTP listener is ready; the runner retries declaration.
func StartBuiltinRunner(ctx context.Context) error {
	if !setting.Actions.Enabled || !setting.Actions.BuiltinRunnerEnabled {
		return nil
	}

	registration, err := createBuiltinRunnerRegistration(ctx)
	if err != nil {
		return fmt.Errorf("create built-in runner registration: %w", err)
	}

	baseDir := filepath.Join(setting.AppDataPath, builtinRunnerDirectory)
	handle, err := builtin.Start(ctx, builtin.Options{
		Address:        setting.LocalURL,
		JobInstanceURL: setting.LocalURL,
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
		return err
	}

	go func() {
		<-ctx.Done()
		handle.Close()
	}()
	log.Info("built-in Actions runner %q started", registration.Name)
	return nil
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
