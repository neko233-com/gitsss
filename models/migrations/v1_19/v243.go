// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import "github.com/neko233-com/gitsss/models/db"

func AddExclusiveLabel(x db.EngineMigration) error {
	type Label struct {
		Exclusive bool
	}

	return x.Sync(new(Label))
}
