// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"time"

	user_model "github.com/neko233-com/gitsss/models/user"
	dailyreport_service "github.com/neko233-com/gitsss/services/dailyreport"
)

func registerDailyReport() {
	RegisterTaskFatal("daily_report", &BaseConfig{Enabled: true, Schedule: "* * * * *"}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		_, err := dailyreport_service.Execute(ctx, time.Now(), true)
		return err
	})
}
