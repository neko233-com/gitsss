// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package projectboard parses references used by user and organization boards.
package projectboard

import (
	"fmt"
	"strconv"
	"strings"
)

// IssueRef points at one issue in a repository.
type IssueRef struct {
	Owner  string
	Repo   string
	Number int64
}

// ParseIssueRef accepts owner/repo#123.
func ParseIssueRef(spec string) (IssueRef, error) {
	spec = strings.TrimSpace(spec)
	ownerRepo, numberText, ok := strings.Cut(spec, "#")
	if !ok {
		return IssueRef{}, fmt.Errorf("issue %q must look like owner/repo#123", spec)
	}
	owner, repo, ok := strings.Cut(strings.Trim(ownerRepo, "/"), "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return IssueRef{}, fmt.Errorf("issue %q must look like owner/repo#123", spec)
	}
	number, err := strconv.ParseInt(numberText, 10, 64)
	if err != nil || number <= 0 {
		return IssueRef{}, fmt.Errorf("issue %q has no positive number", spec)
	}
	return IssueRef{Owner: owner, Repo: repo, Number: number}, nil
}
