// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/perm"
)

func AddRepoUnitEveryoneAccessMode(x db.EngineMigration) error {
	type RepoUnit struct { //revive:disable-line:exported
		EveryoneAccessMode perm.AccessMode `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(&RepoUnit{})
}
