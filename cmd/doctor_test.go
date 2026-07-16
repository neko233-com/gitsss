// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/services/doctor"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestDoctorRun(t *testing.T) {
	doctor.Register(&doctor.Check{
		Title: "Test Check",
		Name:  "test-check",
		Run:   func(ctx context.Context, logger log.Logger, autofix bool) error { return nil },

		SkipDatabaseInitialization: true,
	})
	app := &cli.Command{
		Commands: []*cli.Command{newDoctorCheckCommand()},
	}
	err := app.Run(t.Context(), []string{"./gitsss", "check", "--run", "test-check"})
	assert.NoError(t, err)
	err = app.Run(t.Context(), []string{"./gitsss", "check", "--run", "no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
	err = app.Run(t.Context(), []string{"./gitsss", "check", "--run", "test-check,no-such"})
	assert.ErrorContains(t, err, `unknown checks: "no-such"`)
}
