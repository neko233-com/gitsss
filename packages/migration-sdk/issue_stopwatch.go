// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// StopWatch represents a running stopwatch of an issue / pr
type StopWatch struct {
	Created       time.Time `json:"created"`
	Seconds       int64     `json:"seconds"`
	Duration      string    `json:"duration"`
	IssueIndex    int64     `json:"issue_index"`
	IssueTitle    string    `json:"issue_title"`
	RepoOwnerName string    `json:"repo_owner_name"`
	RepoName      string    `json:"repo_name"`
}

// ListStopwatchesOptions options for listing stopwatches
type ListStopwatchesOptions struct {
	ListOptions
}

// ListMyStopwatches list all stopwatches with pagination
func (c *IssuesService) ListMyStopwatches(ctx context.Context, opt ListStopwatchesOptions) ([]*StopWatch, *Response, error) {
	link, _ := url.Parse("/user/stopwatches")
	opt.setDefaults()
	link.RawQuery = opt.getURLQuery().Encode()
	stopwatches := make([]*StopWatch, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &stopwatches)
	return stopwatches, resp, err
}

// DeleteIssueStopwatch delete / cancel a specific stopwatch
func (c *IssuesService) DeleteIssueStopwatch(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/delete", owner, repo, index), nil, nil)
}

// StartIssueStopWatch starts a stopwatch for an existing issue for a given
// repository
func (c *IssuesService) StartIssueStopWatch(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/start", owner, repo, index), nil, nil)
}

// StopIssueStopWatch stops an existing stopwatch for an issue in a given
// repository
func (c *IssuesService) StopIssueStopWatch(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/stop", owner, repo, index), nil, nil)
}
