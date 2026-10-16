// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "github.com/neko233-com/gitsss/models/db"

func AddRepoAdminChangeTeamAccessColumnForUser(x db.EngineMigration) error {
	type User struct {
		RepoAdminChangeTeamAccess bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(User))
}
