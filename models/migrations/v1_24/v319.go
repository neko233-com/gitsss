// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"github.com/neko233-com/gitsss/models/db"

	"xorm.io/xorm"
)

func AddExclusiveOrderColumnToLabelTable(x db.EngineMigration) error {
	type Label struct {
		ExclusiveOrder int `xorm:"DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Label))
	return err
}
