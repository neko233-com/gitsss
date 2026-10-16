// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"
)

// ActionsArtifact represents an Actions artifact.
type ActionsArtifact struct {
	ID                 int64               `json:"id"`
	Name               string              `json:"name"`
	SizeInBytes        int64               `json:"size_in_bytes"`
	URL                string              `json:"url"`
	ArchiveDownloadURL string              `json:"archive_download_url"`
	Expired            bool                `json:"expired"`
	WorkflowRun        *ActionsWorkflowRun `json:"workflow_run"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	ExpiresAt          time.Time           `json:"expires_at"`
}

// Deprecated: use ActionsArtifact instead.
type ActionArtifact = ActionsArtifact

// ActionsArtifactsResponse contains a page of artifacts.
type ActionsArtifactsResponse struct {
	Artifacts  []*ActionsArtifact `json:"artifacts"`
	TotalCount int64              `json:"total_count"`
}

// Deprecated: use ActionsArtifactsResponse instead.
type ActionArtifactsResponse = ActionsArtifactsResponse

// ListActionsArtifactsOptions controls artifact listing requests.
type ListActionsArtifactsOptions struct {
	ListOptions
	Name string
}

// Deprecated: use ListActionsArtifactsOptions instead.
type ListActionArtifactsOptions = ListActionsArtifactsOptions

// QueryEncode turns the artifact list options into a query string.
func (opt *ListActionsArtifactsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Name != "" {
		query.Add("name", opt.Name)
	}
	return query.Encode()
}

func (c *ActionsService) listActionArtifacts(ctx context.Context, path string, opt ListActionsArtifactsOptions) (*ActionsArtifactsResponse, *Response, error) {
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsArtifactsResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// ListRepoActionRunArtifacts lists artifacts for one workflow run.
func (c *ActionsService) ListRepoRunArtifacts(ctx context.Context, owner, repo string, runID int64, opt ListActionsArtifactsOptions) (*ActionsArtifactsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionArtifacts(ctx, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/artifacts", owner, repo, runID), opt)
}

// ListRepoActionArtifacts lists repository artifacts.
func (c *ActionsService) ListRepoArtifacts(ctx context.Context, owner, repo string, opt ListActionsArtifactsOptions) (*ActionsArtifactsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionArtifacts(ctx, fmt.Sprintf("/repos/%s/%s/actions/artifacts", owner, repo), opt)
}

// GetRepoActionArtifact gets one repository artifact.
func (c *ActionsService) GetRepoArtifact(ctx context.Context, owner, repo string, artifactID int64) (*ActionsArtifact, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	artifact := new(ActionsArtifact)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d", owner, repo, artifactID), jsonHeader, nil, artifact)
	return artifact, resp, err
}

// DeleteRepoActionArtifact deletes one repository artifact.
func (c *ActionsService) DeleteRepoArtifact(ctx context.Context, owner, repo string, artifactID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d", owner, repo, artifactID), nil, nil)
}

// GetRepoActionArtifactArchive downloads one repository artifact zip archive.
func (c *ActionsService) GetRepoArtifactArchive(ctx context.Context, owner, repo string, artifactID int64) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d/zip", owner, repo, artifactID), nil, nil)
}

// GetRepoActionArtifactArchiveReader returns a reader for one repository artifact zip archive.
func (c *ActionsService) GetRepoArtifactArchiveReader(ctx context.Context, owner, repo string, artifactID int64) (io.ReadCloser, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getResponseReader(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d/zip", owner, repo, artifactID), nil, nil)
}
