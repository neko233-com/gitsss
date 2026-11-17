// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/git/gitcmd"
	"github.com/neko233-com/gitsss/modules/gitrepo"
)

const (
	maxReportDiffBytes = 256 << 10
	maxCommitDiffBytes = 16 << 10
	maxReportCommits   = 256
)

// cappedDiff retains a bounded prefix while allowing Git to finish normally.
type cappedDiff struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (w *cappedDiff) Write(p []byte) (int, error) {
	n := len(p)
	remaining := w.limit - w.buffer.Len()
	if remaining < len(p) {
		w.truncated = true
		p = p[:max(remaining, 0)]
	}
	_, _ = w.buffer.Write(p)
	return n, nil
}

func readCommitDiff(ctx context.Context, repo gitrepo.Repository, commitID string, limit int) (string, bool, error) {
	if !objectIDPattern.MatchString(commitID) || limit <= 0 {
		return "", false, errors.New("invalid commit diff request")
	}
	output := &cappedDiff{limit: limit}
	cmd := gitcmd.NewCommand("show", "--format=", "--first-parent", "-m", "--root", "--stat=120,80,80", "--patch", "--unified=2", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames").
		AddDynamicArguments(commitID + "^{commit}").AddDashesAndList().WithTimeout(30 * time.Second).WithStdoutCopy(output)
	if err := gitrepo.RunCmdWithStderr(ctx, repo, cmd); err != nil {
		return "", false, errors.New("cannot read commit diff")
	}
	if !utf8.Valid(output.buffer.Bytes()) {
		output.truncated = true
	}
	return strings.ToValidUTF8(output.buffer.String(), ""), output.truncated, nil
}

func addCommitDiffs(ctx context.Context, orgID int64, grouped map[string][]dailyreport.AttributedCommit) error {
	accounts := make([]string, 0, len(grouped))
	for account := range grouped {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)
	repos := make(map[int64]*repo_model.Repository)
	for _, account := range accounts {
		commits := grouped[account]
		if len(commits) > maxReportCommits {
			return fmt.Errorf("daily report exceeds %d commits for one account", maxReportCommits)
		}
		if len(commits) == 0 {
			continue
		}
		limit := min(maxCommitDiffBytes, maxReportDiffBytes/len(commits))
		for i := range commits {
			repo := repos[commits[i].RepositoryID]
			if repo == nil {
				var err error
				repo, err = repo_model.GetRepositoryByID(ctx, commits[i].RepositoryID)
				if err != nil || repo.OwnerID != orgID {
					return errors.New("cannot access organization report repository")
				}
				repos[repo.ID] = repo
			}
			diff, truncated, err := readCommitDiff(ctx, repo, commits[i].ID, limit)
			if err != nil {
				return fmt.Errorf("cannot read report diff for repository %d commit %s", repo.ID, commits[i].ID)
			}
			commits[i].Diff, commits[i].DiffTruncated = diff, truncated
		}
	}
	return nil
}
