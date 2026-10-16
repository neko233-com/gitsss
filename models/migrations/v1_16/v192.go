// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/migrations/base"
)

func RecreateIssueResourceIndexTable(x db.EngineMigration) error {
	type IssueIndex struct {
		GroupID  int64 `xorm:"pk"`
		MaxIndex int64 `xorm:"index"`
	}

	return base.RecreateTables(new(IssueIndex))(x)
}
