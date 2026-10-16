// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import "github.com/neko233-com/gitsss/models/db"

func AddTrustModelToRepository(x db.EngineMigration) error {
	type Repository struct {
		TrustModel int
	}
	return x.Sync(new(Repository))
}
