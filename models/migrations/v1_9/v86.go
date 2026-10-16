// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import "github.com/neko233-com/gitsss/models/db"

func AddHTTPMethodToWebhook(x db.EngineMigration) error {
	type Webhook struct {
		HTTPMethod string `xorm:"http_method DEFAULT 'POST'"`
	}

	return x.Sync(new(Webhook))
}
