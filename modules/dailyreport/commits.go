// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"errors"
	"fmt"
	"strings"
)

// Commit contains the information needed to attribute and summarize a commit.
type Commit struct {
	ID          string
	AuthorEmail string
	Message     string
}

// Push contains commits from a single repository push.
type Push struct {
	RepositoryID int64
	Total        int
	Commits      []Commit
}

// AttributedCommit omits author email from the provider input.
type AttributedCommit struct {
	RepositoryID int64  `json:"repository_id"`
	ID           string `json:"id"`
	Message      string `json:"message"`
}

// GroupCommits attributes complete push records and removes repeat pushes.
// An empty account from resolve denotes an email without a matching account.
func GroupCommits(pushes []Push, accounts map[string]string, resolve func(string) (string, error)) (map[string][]AttributedCommit, error) {
	if resolve == nil {
		return nil, errors.New("missing commit author resolver")
	}
	result := make(map[string][]AttributedCommit)
	seen := make(map[string]bool)
	for _, push := range pushes {
		if push.RepositoryID <= 0 || push.Total != len(push.Commits) {
			return nil, errors.New("incomplete commit records")
		}
		for _, commit := range push.Commits {
			if commit.ID == "" || commit.AuthorEmail == "" {
				return nil, errors.New("invalid commit record")
			}
			key := fmt.Sprintf("%d:%s", push.RepositoryID, commit.ID)
			if seen[key] {
				continue
			}
			seen[key] = true
			account, err := resolve(commit.AuthorEmail)
			if err != nil {
				return nil, errors.New("cannot resolve commit author")
			}
			if !ShouldReport(accounts[account], 1) {
				continue
			}
			result[account] = append(result[account], AttributedCommit{RepositoryID: push.RepositoryID, ID: commit.ID, Message: commit.Message})
		}
	}
	return result, nil
}

// MatchesRoleLabels matches any selected label; disabled filtering includes unlabeled users.
func MatchesRoleLabels(enabled bool, selected, userLabels []string) bool {
	if !enabled {
		return true
	}
	for _, selectedLabel := range selected {
		for _, label := range userLabels {
			if strings.EqualFold(selectedLabel, label) {
				return true
			}
		}
	}
	return false
}
