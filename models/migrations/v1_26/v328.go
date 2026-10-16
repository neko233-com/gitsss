// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"github.com/neko233-com/gitsss/models/db"

	"xorm.io/xorm"
)

func AddTokenPermissionsToActionRunJob(x db.EngineMigration) error {
	type ActionRunJob struct {
		TokenPermissions string `xorm:"JSON TEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}
