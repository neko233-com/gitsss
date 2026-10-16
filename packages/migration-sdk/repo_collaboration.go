// Copyright 2021 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListCollaboratorsOptions options for listing a repository's collaborators
type ListCollaboratorsOptions struct {
	ListOptions
}

// CollaboratorPermissionResult result type for CollaboratorPermission
type CollaboratorPermissionResult struct {
	Permission AccessMode `json:"permission"`
	Role       string     `json:"role_name"`
	User       *User      `json:"user"`
}

// ListCollaborators list a repository's collaborators
func (c *RepositoriesService) ListCollaborators(ctx context.Context, user, repo string, opt ListCollaboratorsOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	collaborators := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/collaborators?%s", user, repo, opt.getURLQuery().Encode()),
		nil, nil, &collaborators)
	return collaborators, resp, err
}

// IsCollaborator check if a user is a collaborator of a repository
func (c *RepositoriesService) IsCollaborator(ctx context.Context, user, repo, collaborator string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "GET", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
	if err != nil {
		return false, resp, err
	}
	if status == 204 {
		return true, resp, nil
	}
	return false, resp, nil
}

// CollaboratorPermission gets collaborator permission of a repository
func (c *RepositoriesService) CollaboratorPermission(ctx context.Context, user, repo, collaborator string) (*CollaboratorPermissionResult, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return nil, nil, err
	}
	rv := new(CollaboratorPermissionResult)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/collaborators/%s/permission", user, repo, collaborator),
		nil,
		nil,
		rv)
	if err != nil {
		return nil, resp, err
	}
	if resp.StatusCode != 200 {
		rv = nil
	}
	return rv, resp, nil
}

// AddCollaboratorOption options when adding a user as a collaborator of a repository
type AddCollaboratorOption struct {
	Permission *AccessMode `json:"permission"`
}

// AccessMode represent the grade of access you have to something
type AccessMode string

const (
	// AccessModeNone no access
	AccessModeNone AccessMode = "none"
	// AccessModeRead read access
	AccessModeRead AccessMode = "read"
	// AccessModeWrite write access
	AccessModeWrite AccessMode = "write"
	// AccessModeAdmin admin access
	AccessModeAdmin AccessMode = "admin"
	// AccessModeOwner owner
	AccessModeOwner AccessMode = "owner"
)

// Validate the AddCollaboratorOption struct
func (opt *AddCollaboratorOption) Validate() error {
	if opt.Permission != nil {
		if *opt.Permission == AccessModeOwner {
			*opt.Permission = AccessModeAdmin
			return nil
		}
		if *opt.Permission == AccessModeNone {
			opt.Permission = nil
			return nil
		}
		if *opt.Permission != AccessModeRead && *opt.Permission != AccessModeWrite && *opt.Permission != AccessModeAdmin {
			return fmt.Errorf("permission mode invalid")
		}
	}
	return nil
}

// AddCollaborator add some user as a collaborator of a repository
func (c *RepositoriesService) AddCollaborator(ctx context.Context, user, repo, collaborator string, opt AddCollaboratorOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), jsonHeader, bytes.NewReader(body))
}

// DeleteCollaborator remove a collaborator from a repository
func (c *RepositoriesService) DeleteCollaborator(ctx context.Context, user, repo, collaborator string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &collaborator); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/collaborators/%s", user, repo, collaborator), nil, nil)
}

// GetReviewers return all users that can be requested to review in this repo
func (c *RepositoriesService) GetReviewers(ctx context.Context, user, repo string) ([]*User, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	reviewers := make([]*User, 0, 5)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/reviewers", user, repo), nil, nil, &reviewers)
	return reviewers, resp, err
}

// GetAssignees return all users that have write access and can be assigned to issues
func (c *RepositoriesService) GetAssignees(ctx context.Context, user, repo string) ([]*User, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	assignees := make([]*User, 0, 5)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/assignees", user, repo), nil, nil, &assignees)
	return assignees, resp, err
}

// GetRepoTeams return teams from a repository
func (c *RepositoriesService) GetRepoTeams(ctx context.Context, user, repo string) ([]*Team, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	teams := make([]*Team, 0, 5)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/teams", user, repo), nil, nil, &teams)
	return teams, resp, err
}

// AddRepoTeam add a team to a repository
func (c *RepositoriesService) AddRepoTeam(ctx context.Context, user, repo, team string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil)
}

// RemoveRepoTeam delete a team from a repository
func (c *RepositoriesService) RemoveRepoTeam(ctx context.Context, user, repo, team string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil)
}

// CheckRepoTeam check if team is assigned to repo by name and return it.
// If not assigned, it will return nil.
func (c *RepositoriesService) CheckRepoTeam(ctx context.Context, user, repo, team string) (*Team, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &team); err != nil {
		return nil, nil, err
	}
	t := new(Team)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/teams/%s", user, repo, team), nil, nil, &t)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		// if not found it's not an error, it indicates it's not assigned
		return nil, resp, nil
	}
	return t, resp, err
}
