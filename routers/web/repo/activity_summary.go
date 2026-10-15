// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/activityreport"
	"github.com/neko233-com/gitsss/modules/git/gitcmd"
	"github.com/neko233-com/gitsss/modules/gitrepo"
	"github.com/neko233-com/gitsss/services/context"
)

// ActivitySummary returns signed-in commit counts by day, week, and month.
// Callers must already have read access to the repository code.
func ActivitySummary(ctx *context.Context) {
	since := time.Now().AddDate(-1, -1, 0).UTC().Format("2006-01-02")
	stdout, _, runErr := gitrepo.RunCmdString(ctx, ctx.Repo.Repository,
		gitcmd.NewCommand("log").AddDynamicArguments("--since="+since, "--format=%ct", "--all"))
	if runErr != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "activity summary failed"})
		return
	}
	var times []int64
	for _, line := range strings.Fields(stdout) {
		ts, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			continue
		}
		times = append(times, ts)
	}
	if len(times) > 200000 {
		times = times[:200000]
	}
	ctx.JSON(http.StatusOK, activityreport.SummarizeUnix(times, time.Now()))
}
