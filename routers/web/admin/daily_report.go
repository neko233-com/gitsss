// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"github.com/neko233-com/gitsss/services/context"
)

// DailyReportRetired prevents obsolete global settings and execution from sending reports.
func DailyReportRetired(ctx *context.Context) {
	ctx.PlainText(http.StatusGone, "Daily reports have moved to organization settings.")
}
