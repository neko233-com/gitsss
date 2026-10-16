// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// BranchProtection represents a branch protection for a repository
type BranchProtection struct {
	BranchName                    string    `json:"branch_name"`
	RuleName                      string    `json:"rule_name"`
	EnablePush                    bool      `json:"enable_push"`
	EnablePushWhitelist           bool      `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string  `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string  `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool      `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool      `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string  `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string  `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool      `json:"enable_status_check"`
	StatusCheckContexts           []string  `json:"status_check_contexts"`
	RequiredApprovals             int64     `json:"required_approvals"`
	EnableApprovalsWhitelist      bool      `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string  `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string  `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool      `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool      `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool      `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool      `json:"dismiss_stale_approvals"`
	RequireSignedCommits          bool      `json:"require_signed_commits"`
	ProtectedFilePatterns         string    `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string    `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       bool      `json:"block_admin_merge_override"`
	Created                       time.Time `json:"created_at"`
	Updated                       time.Time `json:"updated_at"`
}

// CreateBranchProtectionOption options for creating a branch protection
type CreateBranchProtectionOption struct {
	BranchName                    string   `json:"branch_name"`
	RuleName                      string   `json:"rule_name"`
	EnablePush                    bool     `json:"enable_push"`
	EnablePushWhitelist           bool     `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool     `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool     `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool     `json:"enable_status_check"`
	StatusCheckContexts           []string `json:"status_check_contexts"`
	RequiredApprovals             int64    `json:"required_approvals"`
	EnableApprovalsWhitelist      bool     `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool     `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool     `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool     `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool     `json:"dismiss_stale_approvals"`
	RequireSignedCommits          bool     `json:"require_signed_commits"`
	ProtectedFilePatterns         string   `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string   `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       bool     `json:"block_admin_merge_override"`
}

// EditBranchProtectionOption options for editing a branch protection
type EditBranchProtectionOption struct {
	EnablePush                    *bool    `json:"enable_push"`
	EnablePushWhitelist           *bool    `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       *bool    `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          *bool    `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string `json:"merge_whitelist_teams"`
	EnableStatusCheck             *bool    `json:"enable_status_check"`
	StatusCheckContexts           []string `json:"status_check_contexts"`
	RequiredApprovals             *int64   `json:"required_approvals"`
	EnableApprovalsWhitelist      *bool    `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        *bool    `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests *bool    `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         *bool    `json:"block_on_outdated_branch"`
	DismissStaleApprovals         *bool    `json:"dismiss_stale_approvals"`
	RequireSignedCommits          *bool    `json:"require_signed_commits"`
	ProtectedFilePatterns         *string  `json:"protected_file_patterns"`
	UnprotectedFilePatterns       *string  `json:"unprotected_file_patterns"`
	BlockAdminMergeOverride       *bool    `json:"block_admin_merge_override"`
}

// ListBranchProtectionsOptions list branch protection options
type ListBranchProtectionsOptions struct {
	ListOptions
}

// ListBranchProtections list branch protections for a repo
func (c *RepositoriesService) ListBranchProtections(ctx context.Context, owner, repo string, opt ListBranchProtectionsOptions) ([]*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	bps := make([]*BranchProtection, 0, opt.PageSize)
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &bps)
	return bps, resp, err
}

// GetBranchProtection gets a branch protection
func (c *RepositoriesService) GetBranchProtection(ctx context.Context, owner, repo, name string) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil, bp)
	return bp, resp, err
}

// CreateBranchProtection creates a branch protection for a repo
func (c *RepositoriesService) CreateBranchProtection(ctx context.Context, owner, repo string, opt CreateBranchProtectionOption) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/branch_protections", owner, repo), jsonHeader, bytes.NewReader(body), bp)
	return bp, resp, err
}

// EditBranchProtection edits a branch protection for a repo
func (c *RepositoriesService) EditBranchProtection(ctx context.Context, owner, repo, name string, opt EditBranchProtectionOption) (*BranchProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	bp := new(BranchProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, bytes.NewReader(body), bp)
	return bp, resp, err
}

// DeleteBranchProtection deletes a branch protection for a repo
func (c *RepositoriesService) DeleteBranchProtection(ctx context.Context, owner, repo, name string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &name); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/branch_protections/%s", owner, repo, name), jsonHeader, nil)
}

// TagProtection represents a tag protection for a repository
type TagProtection struct {
	ID                 int64     `json:"id"`
	NamePattern        string    `json:"name_pattern"`
	WhitelistUsernames []string  `json:"whitelist_usernames"`
	WhitelistTeams     []string  `json:"whitelist_teams"`
	Created            time.Time `json:"created_at"`
	Updated            time.Time `json:"updated_at"`
}

// CreateTagProtectionOption options for creating a tag protection
type CreateTagProtectionOption struct {
	NamePattern        string   `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}

// EditTagProtectionOption options for editing a tag protection
type EditTagProtectionOption struct {
	NamePattern        *string  `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}

// ListRepoTagsOptions options for listing a repository's tags
type ListRepoTagProtectionsOptions struct {
	ListOptions
}

// ListTagProtection list tag protections for a repository
func (c *RepositoriesService) ListTagProtection(ctx context.Context, owner, repo string, opt ListRepoTagProtectionsOptions) ([]*TagProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_23_0); err != nil {
		return nil, nil, err
	}

	tps := make([]*TagProtection, 0, opt.PageSize)
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/tag_protections", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &tps)
	return tps, resp, err
}

// GetTagProtection gets a tag protection
func (c *RepositoriesService) GetTagProtection(ctx context.Context, owner, repo string, id int64) (*TagProtection, *Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_23_0); err != nil {
		return nil, nil, err
	}
	tp := new(TagProtection)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, nil, tp)
	return tp, resp, err
}

// CreateTagProtection creates a tag protection for a repo
func (c *RepositoriesService) CreateTagProtection(ctx context.Context, owner, repo string, opt CreateTagProtectionOption) (*TagProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_23_0); err != nil {
		return nil, nil, err
	}

	tp := new(TagProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/tag_protections", owner, repo), jsonHeader, bytes.NewReader(body), tp)
	return tp, resp, err
}

// EditTagProtection edits a tag protection for a repo
func (c *RepositoriesService) EditTagProtection(ctx context.Context, owner, repo string, id int64, opt EditTagProtectionOption) (*TagProtection, *Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_23_0); err != nil {
		return nil, nil, err
	}

	tp := new(TagProtection)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), tp)
	return tp, resp, err
}

// DeleteTagProtection deletes a tag protection for a repo
func (c *RepositoriesService) DeleteTagProtection(ctx context.Context, owner, repo string, id int64) (*Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_23_0); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, nil)
}

// UpdateBranchProtectionPriorities updates the priorities of branch protection rules
func (c *RepositoriesService) UpdateBranchProtectionPriorities(ctx context.Context, owner, repo string, ids []int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	opt := map[string][]int64{"ids": ids}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/branch_protections/priority", owner, repo),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
