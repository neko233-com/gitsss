// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/migrations/v1_27"
	"github.com/neko233-com/gitsss/models/organization"

	"xorm.io/xorm"
)

type gitsssSchemaVersion struct {
	ID      int64 `xorm:"pk"`
	Version int64
}

// repairLegacyGitSSS332 restores the upstream task displaced by the old project migration.
func repairLegacyGitSSS332(ctx context.Context, x db.EngineMigration, version int64) error {
	if version != 332 {
		return nil
	}
	fields, err := x.Dialect().IsColumnExist(x.DB(), ctx, "project", "fields_json")
	if err != nil {
		return err
	}
	legacy, err := x.Dialect().IsColumnExist(x.DB(), ctx, "action_run", "concurrency_group")
	if err != nil {
		return err
	}
	if fields && legacy {
		if err := v1_27.AddActionRunAttemptModel(x); err != nil {
			return fmt.Errorf("restore upstream migration 331 for legacy GitSSS: %w", err)
		}
	}
	attempts, err := x.IsTableExist("action_run_attempt")
	if err != nil {
		return err
	}
	if !attempts {
		return errors.New("database version 332 has no Actions attempts table; restore a consistent backup before upgrading")
	}
	return nil
}

func migrateGitSSSExtensions(x db.EngineMigration) error {
	if err := x.Sync(new(gitsssSchemaVersion)); err != nil {
		return err
	}
	version := &gitsssSchemaVersion{ID: 1}
	has, err := x.Get(version)
	if err != nil {
		return err
	}
	if version.Version > 2 {
		return fmt.Errorf("GitSSS extension schema %d is newer than supported version 2", version.Version)
	}
	if version.Version == 2 {
		return nil
	}
	type Project struct {
		FieldsJSON string `xorm:"TEXT"`
	}
	type Team struct {
		DisplayName string
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true, IgnoreConstrains: true}, new(Project), new(Team), new(organization.OrgTask)); err != nil {
		return err
	}
	version.Version = 2
	if has {
		_, err = x.ID(version.ID).Update(version)
	} else {
		_, err = x.Insert(version)
	}
	return err
}
