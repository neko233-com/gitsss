// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ActionsWorkflowJob represents a job within a workflow run
type ActionsWorkflowJob struct {
	ID          int64                  `json:"id"`
	RunID       int64                  `json:"run_id"`
	RunURL      string                 `json:"run_url"`
	RunAttempt  int64                  `json:"run_attempt"`
	Name        string                 `json:"name"`
	HeadBranch  string                 `json:"head_branch,omitempty"`
	HeadSha     string                 `json:"head_sha"`
	Status      string                 `json:"status"`
	Conclusion  string                 `json:"conclusion,omitempty"`
	URL         string                 `json:"url"`
	HTMLURL     string                 `json:"html_url"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at"`
	RunnerID    int64                  `json:"runner_id,omitempty"`
	RunnerName  string                 `json:"runner_name,omitempty"`
	Labels      []string               `json:"labels"`
	Steps       []*ActionsWorkflowStep `json:"steps"`
}

// Deprecated: use ActionsWorkflowJob instead.
type ActionWorkflowJob = ActionsWorkflowJob

// ActionsWorkflowJobsResponse holds the response for listing workflow jobs
type ActionsWorkflowJobsResponse struct {
	TotalCount int64                 `json:"total_count"`
	Jobs       []*ActionsWorkflowJob `json:"jobs"`
}

// Deprecated: use ActionsWorkflowJobsResponse instead.
type ActionWorkflowJobsResponse = ActionsWorkflowJobsResponse

// ListRepoActionsJobsOptions options for listing repository action jobs
type ListRepoActionsJobsOptions struct {
	ListOptions
	Status string // Filter by status (pending, queued, in_progress, failure, success, skipped)
}

// Deprecated: use ListRepoActionsJobsOptions instead.
type ListRepoActionJobsOptions = ListRepoActionsJobsOptions

// QueryEncode encodes the options to URL query parameters
func (opt *ListRepoActionsJobsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Status != "" {
		query.Add("status", opt.Status)
	}
	return query.Encode()
}

func (c *ActionsService) listActionJobs(ctx context.Context, path string, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsWorkflowJobsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// ListRepoJobsByRun lists jobs for a workflow run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) ListRepoJobsByRun(ctx context.Context, owner, repo string, runID int64, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsWorkflowJobsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// ListRepoRunJobs lists all run jobs for a repository.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) ListRepoRunJobs(ctx context.Context, owner, repo string, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/jobs", owner, repo))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsWorkflowJobsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// GetRepoRunJob gets a single run job.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) GetRepoRunJob(ctx context.Context, owner, repo string, jobID int64) (*ActionsWorkflowJob, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	job := new(ActionsWorkflowJob)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/jobs/%d", owner, repo, jobID), jsonHeader, nil, job)
	return job, resp, err
}

// GetRepoRunJobLogs gets the logs for a specific run job.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) GetRepoRunJobLogs(ctx context.Context, owner, repo string, jobID int64) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID), nil, nil)
}

// RerunRepoActionRunFailedJobs reruns all failed jobs in a workflow run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) RerunRepoRunFailedJobs(ctx context.Context, owner, repo string, runID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/repos/%s/%s/actions/runs/%d/rerun-failed-jobs", owner, repo, runID), jsonHeader, nil)
}

// RerunRepoRunJob reruns a specific workflow job in a run.
// Requires Gitea 1.26.0 or later.
func (c *ActionsService) RerunRepoRunJob(ctx context.Context, owner, repo string, runID, jobID int64) (*ActionsWorkflowJob, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_26_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	job := new(ActionsWorkflowJob)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs/%d/rerun", owner, repo, runID, jobID), jsonHeader, nil, job)
	return job, resp, err
}

// ListOrgRunJobs lists organization-scoped Actions run jobs.
func (c *ActionsService) ListOrgRunJobs(ctx context.Context, org string, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	return c.listActionJobs(ctx, fmt.Sprintf("/orgs/%s/actions/jobs", org), opt)
}

// ListUserRunJobs lists user-scope Actions run jobs.
func (c *ActionsService) ListUserRunJobs(ctx context.Context, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	return c.listActionJobs(ctx, "/user/actions/jobs", opt)
}

// ListRunJobs lists all Actions run jobs.
func (c *ActionsService) ListRunJobs(ctx context.Context, opt ListRepoActionsJobsOptions) (*ActionsWorkflowJobsResponse, *Response, error) {
	return c.listActionJobs(ctx, "/admin/actions/jobs", opt)
}
