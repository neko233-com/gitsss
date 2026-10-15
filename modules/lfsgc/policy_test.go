// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfsgc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRefusesFreshObjects(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	_, err := Normalize(Request{OlderThan: now.Add(-time.Hour), Now: now})
	require.EqualError(t, err, "LFS GC refuses objects newer than 24h; pass force to override")

	req, err := Normalize(Request{OlderThan: now.Add(-time.Hour), Now: now, Force: true, Repository: "poke-game/client"})
	require.NoError(t, err)
	assert.Equal(t, "poke-game/client", req.Repository)
	assert.True(t, req.OlderThan.Equal(now.Add(-time.Hour)))
}

func TestNormalizeDefaultsToOneWeek(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	req, err := Normalize(Request{Now: now})
	require.NoError(t, err)
	assert.True(t, req.OlderThan.Equal(now.Add(-DefaultAge)))
	report := NewReport(req)
	assert.True(t, report.DryRun)
	assert.Empty(t, report.Repositories)
	assert.NotNil(t, report.Repositories)
	report.Add(RepoResult{Repository: "poke-game/client", Orphaned: 2, Deleted: 1})
	assert.Equal(t, 2, report.Orphaned)
	assert.Equal(t, 1, report.Deleted)
}

func TestParseRepository(t *testing.T) {
	owner, name, err := ParseRepository("  poke-game/client  ")
	require.NoError(t, err)
	assert.Equal(t, "poke-game", owner)
	assert.Equal(t, "client", name)
	_, _, err = ParseRepository("only-one")
	require.Error(t, err)
	_, _, err = ParseRepository("a/b/c")
	require.Error(t, err)
}
