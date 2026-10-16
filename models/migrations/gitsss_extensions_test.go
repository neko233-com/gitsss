// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

func extensionTestEngine(t *testing.T) *xorm.Engine {
	t.Helper()
	previousType := setting.Database.Type
	setting.Database.Type = setting.DatabaseTypeSQLite3
	t.Cleanup(func() { setting.Database.Type = previousType })
	x, err := xorm.NewEngine("sqlite3", ":memory:")
	require.NoError(t, err)
	x.SetMaxOpenConns(1)
	x.SetMapper(names.GonicMapper{})
	t.Cleanup(func() { require.NoError(t, x.Close()) })
	return x
}

func TestGitSSSExtensionsPreserveUpstreamVersionAndLabels(t *testing.T) {
	x := extensionTestEngine(t)
	type Project struct {
		ID         int64 `xorm:"pk"`
		FieldsJSON string
	}
	type Team struct {
		ID          int64 `xorm:"pk"`
		Name        string
		DisplayName string
	}
	require.NoError(t, x.Sync(new(Version), new(Project), new(Team)))
	_, err := x.Insert(&Version{ID: 1, Version: 343}, &Project{ID: 1, FieldsJSON: `{"priority":"high"}`}, &Team{ID: 1, Name: "Owners", DisplayName: "Administrators"})
	require.NoError(t, err)
	require.NoError(t, migrateGitSSSExtensions(x))
	require.NoError(t, migrateGitSSSExtensions(x))
	var version Version
	_, err = x.ID(1).Get(&version)
	require.NoError(t, err)
	require.EqualValues(t, 343, version.Version)
	var project Project
	_, err = x.ID(1).Get(&project)
	require.NoError(t, err)
	require.JSONEq(t, `{"priority":"high"}`, project.FieldsJSON)
	var team Team
	_, err = x.ID(1).Get(&team)
	require.NoError(t, err)
	require.Equal(t, "Owners", team.Name)
	require.Equal(t, "Administrators", team.DisplayName)
}

func TestLegacyGitSSS332RestoresDisplacedActionsMigration(t *testing.T) {
	x := extensionTestEngine(t)
	type Project struct {
		ID         int64 `xorm:"pk"`
		FieldsJSON string
	}
	type ActionRun struct {
		ID                int64 `xorm:"pk"`
		RepoID            int64
		Title             string
		ConcurrencyGroup  string
		ConcurrencyCancel bool
	}
	type ActionRunJob struct {
		ID int64 `xorm:"pk"`
	}
	type ActionArtifact struct {
		ID int64 `xorm:"pk"`
	}
	require.NoError(t, x.Sync(new(Project), new(ActionRun), new(ActionRunJob), new(ActionArtifact)))
	_, err := x.Insert(&Project{ID: 1, FieldsJSON: `{"keep":true}`}, &ActionRun{ID: 7, RepoID: 3, Title: "Historical run", ConcurrencyGroup: "legacy"})
	require.NoError(t, err)
	require.NoError(t, repairLegacyGitSSS332(t.Context(), x, 332))
	require.NoError(t, repairLegacyGitSSS332(t.Context(), x, 332))
	has, err := x.IsTableExist("action_run_attempt")
	require.NoError(t, err)
	require.True(t, has)
	legacy, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "action_run", "concurrency_group")
	require.NoError(t, err)
	require.False(t, legacy)
	var title, fields string
	_, err = x.SQL("SELECT title FROM action_run WHERE id = 7").Get(&title)
	require.NoError(t, err)
	require.Equal(t, "Historical run", title)
	_, err = x.SQL("SELECT fields_json FROM project WHERE id = 1").Get(&fields)
	require.NoError(t, err)
	require.Equal(t, `{"keep":true}`, fields)
}

func TestRejectAmbiguousDatabase332(t *testing.T) {
	x := extensionTestEngine(t)
	_, err := x.Exec("CREATE TABLE project (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	_, err = x.Exec("CREATE TABLE action_run (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.ErrorContains(t, repairLegacyGitSSS332(t.Context(), x, 332), "no Actions attempts table")
}
