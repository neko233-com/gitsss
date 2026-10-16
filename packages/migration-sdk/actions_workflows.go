// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// ActionsWorkflow represents a repository workflow definition.
type ActionsWorkflow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
	HTMLURL   string    `json:"html_url"`
	BadgeURL  string    `json:"badge_url"`
	DeletedAt time.Time `json:"deleted_at"`
}

// Deprecated: use ActionsWorkflow instead.
type ActionWorkflow = ActionsWorkflow

// ActionsWorkflowResponse contains a workflow list response.
type ActionsWorkflowResponse struct {
	Workflows  []*ActionsWorkflow `json:"workflows"`
	TotalCount int64              `json:"total_count"`
}

// Deprecated: use ActionsWorkflowResponse instead.
type ActionWorkflowResponse = ActionsWorkflowResponse

// CreateActionsWorkflowDispatchOption triggers a workflow_dispatch event.
type CreateActionsWorkflowDispatchOption struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// Deprecated: use CreateActionsWorkflowDispatchOption instead.
type CreateActionWorkflowDispatchOption = CreateActionsWorkflowDispatchOption

// Validate checks whether the dispatch payload is valid.
func (opt CreateActionsWorkflowDispatchOption) Validate() error {
	if len(opt.Ref) == 0 {
		return errors.New("empty Ref field")
	}
	return nil
}

// ListRepoActionWorkflows lists repository workflows.
func (c *ActionsService) ListRepoWorkflows(ctx context.Context, owner, repo string) (*ActionsWorkflowResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	workflows := new(ActionsWorkflowResponse)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/workflows", owner, repo), jsonHeader, nil, workflows)
	return workflows, resp, err
}

// GetRepoActionWorkflow gets one repository workflow.
func (c *ActionsService) GetRepoWorkflow(ctx context.Context, owner, repo, workflowID string) (*ActionsWorkflow, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	workflow := new(ActionsWorkflow)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s", owner, repo, workflowID), jsonHeader, nil, workflow)
	return workflow, resp, err
}

// DisableRepoActionWorkflow disables one repository workflow.
func (c *ActionsService) DisableRepoWorkflow(ctx context.Context, owner, repo, workflowID string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/disable", owner, repo, workflowID), nil, nil)
}

// EnableRepoActionWorkflow enables one repository workflow.
func (c *ActionsService) EnableRepoWorkflow(ctx context.Context, owner, repo, workflowID string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/enable", owner, repo, workflowID), nil, nil)
}

// DispatchRepoActionWorkflow dispatches one repository workflow.
func (c *ActionsService) DispatchRepoWorkflow(ctx context.Context, owner, repo, workflowID string, opt CreateActionsWorkflowDispatchOption, returnRunDetails bool) (*RunDetails, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflowID))
	if returnRunDetails {
		link.RawQuery = url.Values{"return_run_details": []string{"true"}}.Encode()
		details := new(RunDetails)
		resp, err := c.getParsedResponse(ctx, "POST", link.String(), jsonHeader, bytes.NewReader(body), details)
		return details, resp, err
	}

	resp, err := c.doRequestWithStatusHandle(ctx, "POST", link.String(), jsonHeader, bytes.NewReader(body))
	return nil, resp, err
}
