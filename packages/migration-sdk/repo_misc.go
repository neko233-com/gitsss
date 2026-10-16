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
)

// IssueConfigValidation represents the validation result for issue config
type IssueConfigValidation struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateIssueConfig validates the issue config file for a repository
func (c *RepositoriesService) ValidateIssueConfig(ctx context.Context, owner, repo string) (*IssueConfigValidation, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	result := new(IssueConfigValidation)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issue_config/validate", owner, repo),
		jsonHeader, nil, &result)
	return result, resp, err
}

// --- Pin allowed ---

// NewIssuePinsAllowed represents whether new issue/PR pins are allowed
type NewIssuePinsAllowed struct {
	Issues       bool `json:"issues"`
	PullRequests bool `json:"pull_requests"`
}

// CheckPinAllowed checks if the current user can pin issues or PRs
func (c *RepositoriesService) CheckPinAllowed(ctx context.Context, owner, repo string) (*NewIssuePinsAllowed, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	result := new(NewIssuePinsAllowed)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/new_pin_allowed", owner, repo),
		jsonHeader, nil, result)
	return result, resp, err
}

// --- Issue config ---

// IssueConfigContactLink represents an issue config contact link.
type IssueConfigContactLink struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	About string `json:"about"`
}

// IssueConfig represents the parsed issue config for a repository.
type IssueConfig struct {
	BlankIssuesEnabled bool                     `json:"blank_issues_enabled"`
	ContactLinks       []IssueConfigContactLink `json:"contact_links"`
}

// GetIssueConfig gets the issue config for a repository.
func (c *RepositoriesService) GetIssueConfig(ctx context.Context, owner, repo string) (*IssueConfig, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	config := new(IssueConfig)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/issue_config", owner, repo), jsonHeader, nil, config)
	return config, resp, err
}

// --- Licenses ---

// GetRepoLicenses gets detected licenses for a repository.
func (c *RepositoriesService) GetRepoLicenses(ctx context.Context, owner, repo string) ([]string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	licenses := make([]string, 0, 2)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/licenses", owner, repo), jsonHeader, nil, &licenses)
	return licenses, resp, err
}

// --- Signing keys ---

// GetRepoSigningKeyGPG gets the repository signing GPG public key.
func (c *RepositoriesService) GetRepoSigningKeyGPG(ctx context.Context, owner, repo string) (string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return "", nil, err
	}
	key, resp, err := c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/signing-key.gpg", owner, repo), nil, nil)
	return string(key), resp, err
}

// GetRepoSigningKeySSH gets the repository signing SSH public key.
func (c *RepositoriesService) GetRepoSigningKeySSH(ctx context.Context, owner, repo string) (string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return "", nil, err
	}
	key, resp, err := c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/signing-key.pub", owner, repo), nil, nil)
	return string(key), resp, err
}

// --- Subscribers ---

// ListRepoSubscribers lists repository watchers.
func (c *RepositoriesService) ListRepoSubscribers(ctx context.Context, owner, repo string, opt ListOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	subscribers := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/subscribers?%s", owner, repo, opt.getURLQuery().Encode()), jsonHeader, nil, &subscribers)
	return subscribers, resp, err
}

// --- Diff patch ---

// ApplyDiffPatchFileOptions applies a patch against repository contents.
type ApplyDiffPatchFileOptions struct {
	FileOptions
	Content string `json:"content"`
}

// Validate checks whether the patch payload is valid.
func (opt ApplyDiffPatchFileOptions) Validate() error {
	if len(opt.Content) == 0 {
		return errors.New("empty Content field")
	}
	return nil
}

// ApplyRepoDiffPatch applies a patch to repository contents.
func (c *RepositoriesService) ApplyRepoDiffPatch(ctx context.Context, owner, repo string, opt ApplyDiffPatchFileOptions) (*FileResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FileResponse)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/diffpatch", owner, repo), jsonHeader, bytes.NewReader(body), result)
	return result, resp, err
}
