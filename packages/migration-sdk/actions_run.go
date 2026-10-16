// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ActionsTask represents a workflow run task (from /actions/tasks endpoint)
// This is the format returned by older Gitea versions
type ActionsTask struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"` // Workflow name
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	RunNumber    int64     `json:"run_number"`
	Event        string    `json:"event"`
	DisplayTitle string    `json:"display_title"` // PR title or commit message
	Status       string    `json:"status"`
	WorkflowID   string    `json:"workflow_id"` // e.g. "ci.yml"
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Deprecated: use ActionsTask instead.
type ActionTask = ActionsTask

// ActionsTaskResponse holds the response for listing action tasks
type ActionsTaskResponse struct {
	TotalCount   int64          `json:"total_count"`
	WorkflowRuns []*ActionsTask `json:"workflow_runs"`
}

// Deprecated: use ActionsTaskResponse instead.
type ActionTaskResponse = ActionsTaskResponse

// ActionsWorkflowRun represents a workflow run (from /actions/runs endpoint)
// This is the format returned by newer Gitea versions
type ActionsWorkflowRun struct {
	ID             int64       `json:"id"`
	DisplayTitle   string      `json:"display_title"`
	Event          string      `json:"event"`
	HeadBranch     string      `json:"head_branch,omitempty"`
	HeadSha        string      `json:"head_sha"`
	Path           string      `json:"path"`
	RunAttempt     int64       `json:"run_attempt"`
	RunNumber      int64       `json:"run_number"`
	Status         string      `json:"status"`
	Conclusion     string      `json:"conclusion,omitempty"`
	URL            string      `json:"url"`
	HTMLURL        string      `json:"html_url"`
	StartedAt      time.Time   `json:"started_at"`
	CompletedAt    time.Time   `json:"completed_at"`
	Actor          *User       `json:"actor,omitempty"`
	TriggerActor   *User       `json:"trigger_actor,omitempty"`
	Repository     *Repository `json:"repository,omitempty"`
	HeadRepository *Repository `json:"head_repository,omitempty"`
	RepositoryID   int64       `json:"repository_id,omitempty"`
}

// Deprecated: use ActionsWorkflowRun instead.
type ActionWorkflowRun = ActionsWorkflowRun

// ActionsWorkflowRunsResponse holds the response for listing workflow runs
type ActionsWorkflowRunsResponse struct {
	TotalCount   int64                 `json:"total_count"`
	WorkflowRuns []*ActionsWorkflowRun `json:"workflow_runs"`
}

// Deprecated: use ActionsWorkflowRunsResponse instead.
type ActionWorkflowRunsResponse = ActionsWorkflowRunsResponse

// ActionsWorkflowStep represents a step within a job
type ActionsWorkflowStep struct {
	Name        string    `json:"name"`
	Number      int64     `json:"number"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// Deprecated: use ActionsWorkflowStep instead.
type ActionWorkflowStep = ActionsWorkflowStep

// ListRepoActionsRunsOptions options for listing repository action runs
type ListRepoActionsRunsOptions struct {
	ListOptions
	Branch  string // Filter by branch
	Event   string // Filter by triggering event
	Status  string // Filter by status (pending, queued, in_progress, failure, success, skipped)
	Actor   string // Filter by actor (user who triggered the run)
	HeadSHA string // Filter by the SHA of the head commit
}

// Deprecated: use ListRepoActionsRunsOptions instead.
type ListRepoActionRunsOptions = ListRepoActionsRunsOptions

// QueryEncode encodes the options to URL query parameters
func (opt *ListRepoActionsRunsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Branch != "" {
		query.Add("branch", opt.Branch)
	}
	if opt.Event != "" {
		query.Add("event", opt.Event)
	}
	if opt.Status != "" {
		query.Add("status", opt.Status)
	}
	if opt.Actor != "" {
		query.Add("actor", opt.Actor)
	}
	if opt.HeadSHA != "" {
		query.Add("head_sha", opt.HeadSHA)
	}
	return query.Encode()
}

func (c *ActionsService) listActionRuns(ctx context.Context, path string, opt ListRepoActionsRunsOptions) (*ActionsWorkflowRunsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsWorkflowRunsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// ListRepoActionRuns lists workflow runs for a repository.
// Requires Gitea 1.26.0 or later. For older versions, use ListRepoActionTasks.
func (c *ActionsService) ListRepoRuns(ctx context.Context, owner, repo string, opt ListRepoActionsRunsOptions) (*ActionsWorkflowRunsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runs", owner, repo))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsWorkflowRunsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// GetRepoActionRun gets a single workflow run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) GetRepoRun(ctx context.Context, owner, repo string, runID int64) (*ActionsWorkflowRun, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	run := new(ActionsWorkflowRun)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), jsonHeader, nil, run)
	return run, resp, err
}

// ListRepoActionTasks lists workflow tasks for a repository (Gitea 1.24.x and earlier)
// Use this for older Gitea versions that don't have /actions/runs endpoint
func (c *ActionsService) ListRepoTasks(ctx context.Context, owner, repo string, opt ListOptions) (*ActionsTaskResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/tasks", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()

	resp := new(ActionsTaskResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// DeleteRepoActionRun deletes a workflow run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) DeleteRepoRun(ctx context.Context, owner, repo string, runID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), jsonHeader, nil)
}

// RerunRepoActionRun reruns an entire workflow run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) RerunRepoRun(ctx context.Context, owner, repo string, runID int64) (*ActionsWorkflowRun, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	run := new(ActionsWorkflowRun)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/actions/runs/%d/rerun", owner, repo, runID), jsonHeader, nil, run)
	return run, resp, err
}

// ListRuns lists all Actions workflow runs.
func (c *ActionsService) ListRuns(ctx context.Context, opt ListRepoActionsRunsOptions) (*ActionsWorkflowRunsResponse, *Response, error) {
	return c.listActionRuns(ctx, "/admin/actions/runs", opt)
}

// RunDetails contains the workflow run identifiers returned by workflow dispatch.
type RunDetails struct {
	WorkflowRunID int64  `json:"workflow_run_id"`
	RunURL        string `json:"run_url"`
	HTMLURL       string `json:"html_url"`
}

// ListOrgActionRuns lists organization-scoped Actions workflow runs.
func (c *ActionsService) ListOrgRuns(ctx context.Context, org string, opt ListRepoActionsRunsOptions) (*ActionsWorkflowRunsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	return c.listActionRuns(ctx, fmt.Sprintf("/orgs/%s/actions/runs", org), opt)
}

// ListUserActionRuns lists user-scope Actions workflow runs.
func (c *ActionsService) ListUserRuns(ctx context.Context, opt ListRepoActionsRunsOptions) (*ActionsWorkflowRunsResponse, *Response, error) {
	return c.listActionRuns(ctx, "/user/actions/runs", opt)
}
