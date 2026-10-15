// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package lfsgc plans LFS metadata garbage collection without moving object paths.
package lfsgc

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// MinAge is the shortest retention that still leaves a window for a push to
	// upload an LFS object before its git pointer lands.
	MinAge = 24 * time.Hour
	// DefaultAge matches the existing doctor check: a week is long enough that an
	// unreferenced object is not an in-flight upload.
	DefaultAge = 7 * 24 * time.Hour
)

// RepoResult is one repository after a scan.
type RepoResult struct {
	Repository string `json:"repository"`
	Total      int64  `json:"total"`
	Orphaned   int    `json:"orphaned"`
	Collected  int    `json:"collected"`
	Deleted    int    `json:"deleted"`
	Stopped    bool   `json:"stopped,omitempty"`
}

// Report is the machine-readable result of a dry-run or an apply.
type Report struct {
	Apply        bool         `json:"apply"`
	DryRun       bool         `json:"dry_run"`
	OlderThan    string       `json:"older_than"`
	Repository   string       `json:"repository,omitempty"`
	Repositories []RepoResult `json:"repositories"`
	Orphaned     int          `json:"orphaned"`
	Deleted      int          `json:"deleted"`
}

// Request is the operator input shared by the CLI, admin page, and API.
type Request struct {
	Apply      bool
	OlderThan  time.Time
	Repository string
	Force      bool
	Now        time.Time
}

// Normalize checks retention and the optional owner/name limit.
func Normalize(req Request) (Request, error) {
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.OlderThan.IsZero() {
		req.OlderThan = req.Now.Add(-DefaultAge)
	}
	if !req.Force && req.OlderThan.After(req.Now.Add(-MinAge)) {
		return Request{}, errors.New("LFS GC refuses objects newer than 24h; pass force to override")
	}
	owner, name, err := ParseRepository(req.Repository)
	if err != nil {
		return Request{}, err
	}
	if owner != "" {
		req.Repository = owner + "/" + name
	}
	return req, nil
}

// ParseRepository accepts an empty spec or owner/name. It does not look up the database.
func ParseRepository(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", nil
	}
	owner, name, ok := strings.Cut(spec, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") || owner == "." || name == "." {
		return "", "", fmt.Errorf("repository %q must be owner/name", spec)
	}
	return owner, name, nil
}

// NewReport starts an empty JSON report. Repositories is an empty slice so agents do not see null.
func NewReport(req Request) Report {
	return Report{
		Apply:        req.Apply,
		DryRun:       !req.Apply,
		OlderThan:    req.OlderThan.UTC().Format(time.RFC3339),
		Repository:   req.Repository,
		Repositories: []RepoResult{},
	}
}

// Add folds one repository into the totals.
func (r *Report) Add(row RepoResult) {
	r.Repositories = append(r.Repositories, row)
	r.Orphaned += row.Orphaned
	r.Deleted += row.Deleted
}
