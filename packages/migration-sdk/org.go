// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
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
)

// Organization represents an organization
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Deprecated: Use Name instead. See https://github.com/go-gitea/gitea/blob/main/modules/structs/org.go#L29
	UserName                  string `json:"username"`
	FullName                  string `json:"full_name"`
	Email                     string `json:"email"`
	AvatarURL                 string `json:"avatar_url"`
	Description               string `json:"description"`
	Website                   string `json:"website"`
	Location                  string `json:"location"`
	Visibility                string `json:"visibility"`
	RepoAdminChangeTeamAccess bool   `json:"repo_admin_change_team_access"`
}

// VisibleType defines the visibility
type VisibleType string

const (
	// VisibleTypePublic Visible for everyone
	VisibleTypePublic VisibleType = "public"

	// VisibleTypeLimited Visible for every connected user
	VisibleTypeLimited VisibleType = "limited"

	// VisibleTypePrivate Visible only for organization's members
	VisibleTypePrivate VisibleType = "private"
)

// ListOrgsOptions options for listing organizations
type ListOrgsOptions struct {
	ListOptions
}

// ListOrgs lists all public organizations
func (c *OrganizationsService) ListOrgs(ctx context.Context, opt ListOrgsOptions) ([]*Organization, *Response, error) {
	opt.setDefaults()
	link, _ := url.Parse("/orgs")
	link.RawQuery = opt.getURLQuery().Encode()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &orgs)
	return orgs, resp, err
}

// ListMyOrgs list all of current user's organizations
func (c *OrganizationsService) ListMyOrgs(ctx context.Context, opt ListOrgsOptions) ([]*Organization, *Response, error) {
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/orgs?%s", opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// ListUserOrgs list all of some user's organizations
func (c *OrganizationsService) ListUserOrgs(ctx context.Context, user string, opt ListOrgsOptions) ([]*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/orgs?%s", user, opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// GetOrg get one organization by name
func (c *OrganizationsService) GetOrg(ctx context.Context, orgname string) (*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/orgs/%s", orgname), nil, nil, org)
	return org, resp, err
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	Name                      string      `json:"username"`
	FullName                  string      `json:"full_name"`
	Email                     string      `json:"email"`
	Description               string      `json:"description"`
	Website                   string      `json:"website"`
	Location                  string      `json:"location"`
	Visibility                VisibleType `json:"visibility"`
	RepoAdminChangeTeamAccess bool        `json:"repo_admin_change_team_access"`
}

// checkVisibilityOpt check if mode exist
func checkVisibilityOpt(v VisibleType) bool {
	return v == VisibleTypePublic || v == VisibleTypeLimited || v == VisibleTypePrivate
}

// Validate the CreateOrgOption struct
func (opt CreateOrgOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("empty org name")
	}
	if len(opt.Visibility) != 0 && !checkVisibilityOpt(opt.Visibility) {
		return fmt.Errorf("invalid visibility option")
	}
	return nil
}

// CreateOrg creates an organization
func (c *OrganizationsService) CreateOrg(ctx context.Context, opt CreateOrgOption) (*Organization, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse(ctx, "POST", "/orgs", jsonHeader, bytes.NewReader(body), org)
	return org, resp, err
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	FullName                  string      `json:"full_name"`
	Email                     string      `json:"email"`
	Description               string      `json:"description"`
	Website                   string      `json:"website"`
	Location                  string      `json:"location"`
	Visibility                VisibleType `json:"visibility"`
	RepoAdminChangeTeamAccess *bool       `json:"repo_admin_change_team_access"`
}

// Validate the EditOrgOption struct
func (opt EditOrgOption) Validate() error {
	if len(opt.Visibility) != 0 && !checkVisibilityOpt(opt.Visibility) {
		return fmt.Errorf("invalid visibility option")
	}
	return nil
}

// EditOrg modify one organization via options
func (c *OrganizationsService) EditOrg(ctx context.Context, orgname string, opt EditOrgOption) (*Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PATCH", fmt.Sprintf("/orgs/%s", orgname), jsonHeader, bytes.NewReader(body))
}

// DeleteOrg deletes an organization
func (c *OrganizationsService) DeleteOrg(ctx context.Context, orgname string) (*Response, error) {
	if err := escapeValidatePathSegments(&orgname); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s", orgname), jsonHeader, nil)
}

// UpdateOrgAvatar updates the organization's avatar
func (c *OrganizationsService) UpdateOrgAvatar(ctx context.Context, org string, opt UpdateUserAvatarOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/orgs/%s/avatar", org),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// DeleteOrgAvatar deletes the organization's avatar
func (c *OrganizationsService) DeleteOrgAvatar(ctx context.Context, org string) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE",
		fmt.Sprintf("/orgs/%s/avatar", org),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// RenameOrgOption options for renaming an organization
type RenameOrgOption struct {
	NewName string `json:"new_name"`
}

// RenameOrg renames an organization
func (c *OrganizationsService) RenameOrg(ctx context.Context, org string, opt RenameOrgOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/orgs/%s/rename", org),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// ListOrgBlocksOptions options for listing organization blocks
type ListOrgBlocksOptions struct {
	ListOptions
}

// ListOrgBlocks lists users blocked by the organization
func (c *OrganizationsService) ListOrgBlocks(ctx context.Context, org string, opt ListOrgBlocksOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/blocks", org))
	link.RawQuery = opt.getURLQuery().Encode()

	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &users)
	return users, resp, err
}

// CheckOrgBlock checks if a user is blocked by the organization
func (c *OrganizationsService) CheckOrgBlock(ctx context.Context, org, username string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "GET",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return false, resp, err
	}
	return status == http.StatusNoContent, resp, nil
}

// BlockOrgUser blocks a user from the organization
func (c *OrganizationsService) BlockOrgUser(ctx context.Context, org, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "PUT",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnblockOrgUser unblocks a user from the organization
func (c *OrganizationsService) UnblockOrgUser(ctx context.Context, org, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
