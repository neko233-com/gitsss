// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GitServiceType represents a git service
type GitServiceType string

const (
	// GitServicePlain represents a plain git service
	GitServicePlain GitServiceType = "git"
	// GitServiceGithub represents github.com
	GitServiceGithub GitServiceType = "github"
	// GitServiceGitlab represents a gitlab service
	GitServiceGitlab GitServiceType = "gitlab"
	// GitServiceGitea represents a gitea service
	GitServiceGitea GitServiceType = "gitea"
	// GitServiceGogs represents a gogs service
	GitServiceGogs GitServiceType = "gogs"
)

// MigrateRepoOption options for migrating a repository from an external service
type MigrateRepoOption struct {
	RepoName  string `json:"repo_name"`
	RepoOwner string `json:"repo_owner"`
	// deprecated use RepoOwner
	RepoOwnerID    int64          `json:"uid"`
	CloneAddr      string         `json:"clone_addr"`
	Service        GitServiceType `json:"service"`
	AuthUsername   string         `json:"auth_username"`
	AuthPassword   string         `json:"auth_password"`
	AuthToken      string         `json:"auth_token"`
	Mirror         bool           `json:"mirror"`
	Private        bool           `json:"private"`
	Description    string         `json:"description"`
	Wiki           bool           `json:"wiki"`
	Milestones     bool           `json:"milestones"`
	Labels         bool           `json:"labels"`
	Issues         bool           `json:"issues"`
	PullRequests   bool           `json:"pull_requests"`
	Releases       bool           `json:"releases"`
	MirrorInterval string         `json:"mirror_interval"`
	LFS            bool           `json:"lfs"`
	LFSEndpoint    string         `json:"lfs_endpoint"`
}

// Validate the MigrateRepoOption struct
func (opt *MigrateRepoOption) Validate(ctx context.Context, c *Client) error {
	// check user options
	if len(opt.CloneAddr) == 0 {
		return fmt.Errorf("clone addr required")
	}
	if len(opt.RepoName) == 0 {
		return fmt.Errorf("repo name required")
	} else if len(opt.RepoName) > 100 {
		return fmt.Errorf("repo name too long")
	}
	if len(opt.Description) > 2048 {
		return fmt.Errorf("description too long")
	}
	switch opt.Service {
	case GitServiceGithub:
		if len(opt.AuthToken) == 0 {
			return fmt.Errorf("github requires token authentication")
		}
	case GitServiceGitlab, GitServiceGitea:
		if len(opt.AuthToken) == 0 {
			return fmt.Errorf("%s requires token authentication", opt.Service)
		}
		// Gitlab is supported since 1.12.0 but api cant handle it until 1.13.0
		// https://github.com/go-gitea/gitea/pull/12672
		if c.checkServerVersionGreaterThanOrEqual(ctx, version1_13_0) != nil {
			return fmt.Errorf("migrate from service %s need gitea >= 1.13.0", opt.Service)
		}
	case GitServiceGogs:
		if len(opt.AuthToken) == 0 {
			return fmt.Errorf("gogs requires token authentication")
		}
		if c.checkServerVersionGreaterThanOrEqual(ctx, version1_14_0) != nil {
			return fmt.Errorf("migrate from service gogs need gitea >= 1.14.0")
		}
	}
	return nil
}

// MigrateRepo migrates a repository from other Git hosting sources for the authenticated user.
//
// To migrate a repository for a organization, the authenticated user must be a
// owner of the specified organization.
func (c *RepositoriesService) MigrateRepo(ctx context.Context, opt MigrateRepoOption) (*Repository, *Response, error) {
	if err := opt.Validate(ctx, c.Client); err != nil {
		return nil, nil, err
	}

	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_13_0); err != nil {
		if len(opt.AuthToken) != 0 {
			// gitea <= 1.12 dont understand AuthToken
			opt.AuthUsername = opt.AuthToken
			opt.AuthPassword, opt.AuthToken = "", ""
		}
		if len(opt.RepoOwner) != 0 {
			// gitea <= 1.12 dont understand RepoOwner
			u, _, err := c.GetUserInfo(ctx, opt.RepoOwner)
			if err != nil {
				return nil, nil, err
			}
			opt.RepoOwnerID = u.ID
		} else if opt.RepoOwnerID == 0 {
			// gitea <= 1.12 require RepoOwnerID
			u, _, err := c.GetMyUserInfo(ctx)
			if err != nil {
				return nil, nil, err
			}
			opt.RepoOwnerID = u.ID
		}
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", "/repos/migrate", jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

type CreatePushMirrorOption struct {
	Interval       string `json:"interval"`
	RemoteAddress  string `json:"remote_address"`
	RemotePassword string `json:"remote_password"`
	RemoteUsername string `json:"remote_username"`
	SyncONCommit   bool   `json:"sync_on_commit"`
}

// PushMirrorResponse returns a git push mirror
type PushMirrorResponse struct {
	Created       string `json:"created"`
	Interval      string `json:"interval"`
	LastError     string `json:"last_error"`
	LastUpdate    string `json:"last_update"`
	RemoteAddress string `json:"remote_address"`
	RemoteName    string `json:"remote_name"`
	RepoName      string `json:"repo_name"`
	SyncONCommit  bool   `json:"sync_on_commit"`
}

// PushMirrors add a push mirror to the repository
func (c *RepositoriesService) PushMirrors(ctx context.Context, user, repo string, opt CreatePushMirrorOption) (*PushMirrorResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(opt)
	if err != nil {
		return nil, nil, err
	}
	pm := new(PushMirrorResponse)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/push_mirrors", user, repo), jsonHeader, bytes.NewReader(body), &pm)
	return pm, resp, err
}

// ListPushMirrors gets all push mirrors of a repository
func (c *RepositoriesService) ListPushMirrors(ctx context.Context, user, repo string, opt ListOptions) ([]*PushMirrorResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	pms := make([]*PushMirrorResponse, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/push_mirrors?%s", user, repo, opt.getURLQuery().Encode()),
		nil, nil, &pms)
	return pms, resp, err
}

// GetPushMirrorByRemoteName get a push mirror of the repository by remote name
func (c *RepositoriesService) GetPushMirrorByRemoteName(ctx context.Context, user, repo, remoteName string) (*PushMirrorResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &remoteName); err != nil {
		return nil, nil, err
	}
	pm := new(PushMirrorResponse)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/push_mirrors/%s", user, repo, remoteName), nil, nil, &pm)
	return pm, resp, err
}

// DeletePushMirror deletes a push mirror from a repository by remote name
func (c *RepositoriesService) DeletePushMirror(ctx context.Context, user, repo, remoteName string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &remoteName); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/push_mirrors/%s", user, repo, remoteName), nil, nil)
}

// TriggerPushMirrorsSync triggers push-mirror syncing for a repository.
func (c *RepositoriesService) TriggerPushMirrorsSync(ctx context.Context, owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/repos/%s/%s/push_mirrors-sync", owner, repo), nil, nil)
}

// TransferRepoOption options when transfer a repository's ownership
type TransferRepoOption struct {
	// required: true
	NewOwner string `json:"new_owner"`
	// ID of the team or teams to add to the repository. Teams can only be added to organization-owned repositories.
	TeamIDs *[]int64 `json:"team_ids"`
}

// TransferRepo transfers the ownership of a repository
func (c *RepositoriesService) TransferRepo(ctx context.Context, owner, reponame string, opt TransferRepoOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &reponame); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/transfer", owner, reponame), jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// AcceptRepoTransfer accepts a repo transfer.
func (c *RepositoriesService) AcceptRepoTransfer(ctx context.Context, owner, reponame string) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &reponame); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_16_0); err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/transfer/accept", owner, reponame), jsonHeader, nil, repo)
	return repo, resp, err
}

// RejectRepoTransfer rejects a repo transfer.
func (c *RepositoriesService) RejectRepoTransfer(ctx context.Context, owner, reponame string) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &reponame); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_16_0); err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/transfer/reject", owner, reponame), jsonHeader, nil, repo)
	return repo, resp, err
}

// ListForksOptions options for listing repository's forks
type ListForksOptions struct {
	ListOptions
}

// ListForks list a repository's forks
func (c *RepositoriesService) ListForks(ctx context.Context, user, repo string, opt ListForksOptions) ([]*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	forks := make([]*Repository, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/forks?%s", user, repo, opt.getURLQuery().Encode()),
		nil, nil, &forks)
	return forks, resp, err
}

// CreateForkOption options for creating a fork
type CreateForkOption struct {
	// organization name, if forking into an organization
	Organization *string `json:"organization"`
	// name of the forked repository
	Name *string `json:"name"`
}

// CreateFork create a fork of a repository
func (c *RepositoriesService) CreateFork(ctx context.Context, user, repo string, form CreateForkOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(form)
	if err != nil {
		return nil, nil, err
	}
	fork := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/forks", user, repo), jsonHeader, bytes.NewReader(body), &fork)
	return fork, resp, err
}

// MergeUpstreamRequest options for merging upstream
type MergeUpstreamRequest struct {
	Branch string `json:"branch"`
	FfOnly bool   `json:"ff_only"`
}

// MergeUpstreamResponse represents the response from merging upstream
type MergeUpstreamResponse struct {
	MergeStyle string `json:"merge_type"`
}

// MergeUpstream merges upstream into a forked repository
func (c *RepositoriesService) MergeUpstream(ctx context.Context, owner, repo string, opt MergeUpstreamRequest) (*MergeUpstreamResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(MergeUpstreamResponse)
	resp, err := c.getParsedResponse(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/merge-upstream", owner, repo),
		jsonHeader, bytes.NewReader(body), result)
	return result, resp, err
}
