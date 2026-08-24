// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBuiltinRunnerRegistration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	oldVersion := setting.AppVer
	oldLabels := setting.Actions.BuiltinRunnerLabels
	setting.AppVer = "test-version"
	setting.Actions.BuiltinRunnerLabels = []string{"self-hosted:host", "ubuntu-latest:host"}
	t.Cleanup(func() {
		setting.AppVer = oldVersion
		setting.Actions.BuiltinRunnerLabels = oldLabels
		_, _ = db.GetEngine(t.Context()).Where("name = ?", builtinRunnerName).Delete(&actions_model.ActionRunner{})
	})

	runner, err := createBuiltinRunnerRegistration(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, runner.Token)
	assert.NotEqual(t, runner.Token, runner.TokenHash)
	assert.Equal(t, setting.AppVer, runner.Version)
	assert.Equal(t, setting.Actions.BuiltinRunnerLabels, runner.AgentLabels)

	stored, err := actions_model.GetRunnerByID(t.Context(), runner.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.Token)
	assert.Equal(t, runner.TokenHash, stored.TokenHash)
}

func TestBuiltinRunnerURLUsesIPv4LoopbackForLocalhost(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:3000/", normalizeBuiltinRunnerURL("http://localhost:3000/"))
	assert.Equal(t, "https://127.0.0.1/", normalizeBuiltinRunnerURL("https://localhost/"))
	assert.Equal(t, "http://gitsss.internal:3000/", normalizeBuiltinRunnerURL("http://gitsss.internal:3000/"))
}

func TestBuiltinRunnerUsesServiceContextAfterStartup(t *testing.T) {
	requestCtx := context.WithValue(t.Context(), struct{ name string }{}, "request")
	serviceCtx := context.WithValue(t.Context(), struct{ name string }{}, "service")

	oldParentCtx := builtinRunnerManager.parentCtx
	builtinRunnerManager.parentCtx = serviceCtx
	t.Cleanup(func() { builtinRunnerManager.parentCtx = oldParentCtx })

	assert.Same(t, serviceCtx, builtinRunnerContext(requestCtx))

	builtinRunnerManager.parentCtx = nil
	assert.Same(t, requestCtx, builtinRunnerContext(requestCtx))
}
