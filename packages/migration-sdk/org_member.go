// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// DeleteOrgMembership remove a member from an organization
func (c *OrganizationsService) DeleteOrgMembership(ctx context.Context, org, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &user); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s/members/%s", org, user), nil, nil)
}

// ListOrgMembershipOption list OrgMembership options
type ListOrgMembershipOption struct {
	ListOptions
}

// ListOrgMembership list an organization's members
func (c *OrganizationsService) ListOrgMembership(ctx context.Context, org string, opt ListOrgMembershipOption) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/members", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &users)
	return users, resp, err
}

// ListPublicOrgMembership list an organization's members
func (c *OrganizationsService) ListPublicOrgMembership(ctx context.Context, org string, opt ListOrgMembershipOption) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/public_members", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &users)
	return users, resp, err
}

// CheckOrgMembership Check if a user is a member of an organization
func (c *OrganizationsService) CheckOrgMembership(ctx context.Context, org, user string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&org, &user); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "GET", fmt.Sprintf("/orgs/%s/members/%s", org, user), nil, nil)
	if err != nil {
		return false, resp, err
	}
	switch status {
	case http.StatusNoContent:
		return true, resp, nil
	case http.StatusNotFound:
		return false, resp, nil
	default:
		return false, resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// CheckPublicOrgMembership Check if a user is a member of an organization
func (c *OrganizationsService) CheckPublicOrgMembership(ctx context.Context, org, user string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&org, &user); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "GET", fmt.Sprintf("/orgs/%s/public_members/%s", org, user), nil, nil)
	if err != nil {
		return false, resp, err
	}
	switch status {
	case http.StatusNoContent:
		return true, resp, nil
	case http.StatusNotFound:
		return false, resp, nil
	default:
		return false, resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// SetPublicOrgMembership publicize/conceal a user's membership
func (c *OrganizationsService) SetPublicOrgMembership(ctx context.Context, org, user string, visible bool) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &user); err != nil {
		return nil, err
	}
	var (
		status int
		err    error
		resp   *Response
	)
	if visible {
		status, resp, err = c.getStatusCode(ctx, "PUT", fmt.Sprintf("/orgs/%s/public_members/%s", org, user), nil, nil)
	} else {
		status, resp, err = c.getStatusCode(ctx, "DELETE", fmt.Sprintf("/orgs/%s/public_members/%s", org, user), nil, nil)
	}
	if err != nil {
		return resp, err
	}
	switch status {
	case http.StatusNoContent:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("forbidden")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}

// OrgPermissions represents the permissions for an user in an organization
type OrgPermissions struct {
	CanCreateRepository bool `json:"can_create_repository"`
	CanRead             bool `json:"can_read"`
	CanWrite            bool `json:"can_write"`
	IsAdmin             bool `json:"is_admin"`
	IsOwner             bool `json:"is_owner"`
}

// GetOrgPermissions returns user permissions for specific organization.
func (c *OrganizationsService) GetOrgPermissions(ctx context.Context, org, user string) (*OrgPermissions, *Response, error) {
	if err := escapeValidatePathSegments(&org, &user); err != nil {
		return nil, nil, err
	}

	perm := &OrgPermissions{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/orgs/%s/permissions", user, org), jsonHeader, nil, &perm)
	if err != nil {
		return nil, resp, err
	}
	return perm, resp, nil
}
