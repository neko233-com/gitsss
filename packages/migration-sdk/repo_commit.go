// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// GitIdentity for a person's identity like an author or committer.
type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Deprecated: use GitIdentity instead.
type Identity = GitIdentity

// CommitMeta contains meta information of a commit in terms of API.
type CommitMeta struct {
	URL     string    `json:"url"`
	SHA     string    `json:"sha"`
	Created time.Time `json:"created"`
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	GitIdentity
	Date string `json:"date"`
}

// RepoCommit contains information of a commit in the context of a repository.
type RepoCommit struct {
	URL          string                     `json:"url"`
	Author       *CommitUser                `json:"author"`
	Committer    *CommitUser                `json:"committer"`
	Message      string                     `json:"message"`
	Tree         *CommitMeta                `json:"tree"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// CommitStats contains stats from a Git commit
type CommitStats struct {
	Total     int `json:"total"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// Commit contains information generated from a Git commit.
type Commit struct {
	*CommitMeta
	HTMLURL    string                 `json:"html_url"`
	RepoCommit *RepoCommit            `json:"commit"`
	Author     *User                  `json:"author"`
	Committer  *User                  `json:"committer"`
	Parents    []*CommitMeta          `json:"parents"`
	Files      []*CommitAffectedFiles `json:"files"`
	Stats      *CommitStats           `json:"stats"`
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	Author    time.Time `json:"author"`
	Committer time.Time `json:"committer"`
}

// CommitAffectedFiles store information about files affected by the commit
type CommitAffectedFiles struct {
	Filename string `json:"filename"`
}

// GetSingleCommit returns a single commit
func (c *RepositoriesService) GetSingleCommit(ctx context.Context, user, repo, commitID string) (*Commit, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &commitID); err != nil {
		return nil, nil, err
	}
	commit := new(Commit)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s", user, repo, commitID), nil, nil, &commit)
	return commit, resp, err
}

// ListCommitOptions list commit options
type ListCommitOptions struct {
	ListOptions
	// SHA or branch to start listing commits from (usually 'master')
	SHA string
	// Path indicates that only commits that include the path's file/dir should be returned.
	Path string
	// Stat includes diff stats for every commit (disable for speedup)
	Stat bool
	// Verification includes verification for every commit (disable for speedup)
	Verification bool
	// Files includes a list of affected files for every commit (disable for speedup)
	Files bool
	// Not is a string used such that commits that match the given specifier will not be listed.
	Not string
}

// QueryEncode turns options into querystring argument
func (opt *ListCommitOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.SHA != "" {
		query.Add("sha", opt.SHA)
	}
	if opt.Path != "" {
		query.Add("path", opt.Path)
	}
	query.Add("stat", strconv.FormatBool(opt.Stat))
	query.Add("verification", strconv.FormatBool(opt.Verification))
	query.Add("files", strconv.FormatBool(opt.Files))
	if opt.Not != "" {
		query.Add("not", opt.Not)
	}
	return query.Encode()
}

// ListRepoCommits return list of commits from a repo
func (c *RepositoriesService) ListRepoCommits(ctx context.Context, user, repo string, opt ListCommitOptions) ([]*Commit, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/commits", user, repo))
	opt.setDefaults()
	commits := make([]*Commit, 0, opt.PageSize)
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &commits)
	return commits, resp, err
}

// GetCommitDiff returns the commit's raw diff.
func (c *RepositoriesService) GetCommitDiff(ctx context.Context, user, repo, commitID string) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_16_0); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s.%s", user, repo, commitID, pullRequestDiffTypeDiff), nil, nil)
}

// GetCommitPatch returns the commit's raw patch.
func (c *RepositoriesService) GetCommitPatch(ctx context.Context, user, repo, commitID string) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_16_0); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s.%s", user, repo, commitID, pullRequestDiffTypePatch), nil, nil)
}

// StatusState holds the state of a Status
// It can be "pending", "success", "error", "failure", and "warning"
type StatusState string

const (
	// StatusPending is for when the Status is Pending
	StatusPending StatusState = "pending"
	// StatusSuccess is for when the Status is Success
	StatusSuccess StatusState = "success"
	// StatusError is for when the Status is Error
	StatusError StatusState = "error"
	// StatusFailure is for when the Status is Failure
	StatusFailure StatusState = "failure"
	// StatusWarning is for when the Status is Warning
	StatusWarning StatusState = "warning"
)

// Status holds a single Status of a single Commit
type Status struct {
	ID          int64       `json:"id"`
	State       StatusState `json:"status"`
	TargetURL   string      `json:"target_url"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	Context     string      `json:"context"`
	Creator     *User       `json:"creator"`
	Created     time.Time   `json:"created_at"`
	Updated     time.Time   `json:"updated_at"`
}

// CreateStatusOption holds the information needed to create a new Status for a Commit
type CreateStatusOption struct {
	State       StatusState `json:"state"`
	TargetURL   string      `json:"target_url"`
	Description string      `json:"description"`
	Context     string      `json:"context"`
}

// CreateStatus creates a new Status for a given Commit
func (c *RepositoriesService) CreateStatus(ctx context.Context, owner, repo, sha string, opts CreateStatusOption) (*Status, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opts)
	if err != nil {
		return nil, nil, err
	}
	status := new(Status)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/statuses/%s", owner, repo, url.QueryEscape(sha)), jsonHeader, bytes.NewReader(body), status)
	return status, resp, err
}

// ListStatusesOption options for listing a repository's commit's statuses
type ListStatusesOption struct {
	ListOptions
}

// ListStatuses returns all statuses for a given Commit by ref
func (c *RepositoriesService) ListStatuses(ctx context.Context, owner, repo, ref string, opt ListStatusesOption) ([]*Status, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &ref); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	statuses := make([]*Status, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s/statuses?%s", owner, repo, ref, opt.getURLQuery().Encode()), jsonHeader, nil, &statuses)
	return statuses, resp, err
}

// CombinedStatus holds the combined state of several statuses for a single commit
type CombinedStatus struct {
	State      StatusState `json:"state"`
	SHA        string      `json:"sha"`
	TotalCount int         `json:"total_count"`
	Statuses   []*Status   `json:"statuses"`
	Repository *Repository `json:"repository"`
	CommitURL  string      `json:"commit_url"`
	URL        string      `json:"url"`
}

// GetCombinedStatus returns the CombinedStatus for a given Commit
func (c *RepositoriesService) GetCombinedStatus(ctx context.Context, owner, repo, ref string) (*CombinedStatus, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &ref); err != nil {
		return nil, nil, err
	}
	status := new(CombinedStatus)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s/status", owner, repo, ref), jsonHeader, nil, status)

	// gitea api return empty body if nothing here jet
	if resp != nil && resp.StatusCode == 200 && err != nil {
		return status, resp, nil
	}

	return status, resp, err
}

// Compare represents a comparison between two commits.
type Compare struct {
	TotalCommits int       `json:"total_commits"` // Total number of commits in the comparison.
	Commits      []*Commit `json:"commits"`       // List of commits in the comparison.
}

// CompareCommits compares two commits in a repository.
func (c *RepositoriesService) CompareCommits(ctx context.Context, user, repo, prev, current string) (*Compare, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &prev, &current); err != nil {
		return nil, nil, err
	}

	basehead := fmt.Sprintf("%s...%s", prev, current)

	apiResp := new(Compare)
	resp, err := c.getParsedResponse(ctx,
		"GET",
		fmt.Sprintf("/repos/%s/%s/compare/%s", user, repo, basehead),
		nil, nil, apiResp,
	)
	return apiResp, resp, err
}
