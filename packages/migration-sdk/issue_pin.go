// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/http"
)

// ListRepoPinnedIssues lists a repo's pinned issues
func (c *IssuesService) ListRepoPinnedIssues(ctx context.Context, owner, repo string) ([]*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	issues := make([]*Issue, 0, 5)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issues/pinned", owner, repo),
		jsonHeader, nil, &issues)
	return issues, resp, err
}

// PinIssue pins an issue
func (c *IssuesService) PinIssue(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnpinIssue unpins an issue
func (c *IssuesService) UnpinIssue(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// MoveIssuePin moves a pinned issue to the given position
func (c *IssuesService) MoveIssuePin(ctx context.Context, owner, repo string, index, position int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "PATCH",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin/%d", owner, repo, index, position),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
