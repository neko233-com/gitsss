// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/setting"
)

func PrependRefsHeadsToIssueRefs(x db.EngineMigration) error {
	var query string

	switch {
	case setting.Database.Type.IsMSSQL():
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' + `ref` WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%'"
	case setting.Database.Type.IsMySQL():
		query = "UPDATE `issue` SET `ref` = CONCAT('refs/heads/', `ref`) WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%';"
	default:
		query = "UPDATE `issue` SET `ref` = 'refs/heads/' || `ref` WHERE `ref` IS NOT NULL AND `ref` <> '' AND `ref` NOT LIKE 'refs/%'"
	}

	_, err := x.Exec(query)
	return err
}
