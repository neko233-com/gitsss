// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/hostmatcher"
	"github.com/neko233-com/gitsss/modules/setting"

	_ "github.com/neko233-com/gitsss/models"
	_ "github.com/neko233-com/gitsss/models/actions"
)

func TestMain(m *testing.M) {
	// for tests, allow only loopback IPs
	setting.Webhook.AllowedHostList = hostmatcher.MatchBuiltinLoopback
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
