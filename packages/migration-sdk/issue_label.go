// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GetIssueLabels get labels of one issue via issue id
func (c *IssuesService) GetIssueLabels(ctx context.Context, owner, repo string, index int64, opts ListLabelsOptions) ([]*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	labels := make([]*Label, 0, 5)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/issues/%d/labels?%s", owner, repo, index, opts.getURLQuery().Encode()), nil, nil, &labels)
	return labels, resp, err
}

// IssueLabelsOption a collection of labels
type IssueLabelsOption struct {
	// list of label IDs
	Labels []int64 `json:"labels"`
}

// AddIssueLabels add one or more labels to one issue
func (c *IssuesService) AddIssueLabels(ctx context.Context, owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	var labels []*Label
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
	return labels, resp, err
}

// ReplaceIssueLabels replace old labels of issue with new labels
func (c *IssuesService) ReplaceIssueLabels(ctx context.Context, owner, repo string, index int64, opt IssueLabelsOption) ([]*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	var labels []*Label
	resp, err := c.getParsedResponse(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), jsonHeader, bytes.NewReader(body), &labels)
	return labels, resp, err
}

// DeleteIssueLabel delete one label of one issue by issue id and label id
// TODO: maybe we need delete by label name and issue id
func (c *IssuesService) DeleteIssueLabel(ctx context.Context, owner, repo string, index, label int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%d", owner, repo, index, label), nil, nil)
}

// ClearIssueLabels delete all the labels of one issue.
func (c *IssuesService) ClearIssueLabels(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, index), nil, nil)
}
