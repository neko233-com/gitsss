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
	"net/url"
)

// CreateRepo create a repo
func (c *AdminService) CreateRepo(ctx context.Context, user string, opt CreateRepoOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/admin/users/%s/repos", user), jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// ListUnadoptedReposOptions options for listing unadopted repositories
type ListUnadoptedReposOptions struct {
	ListOptions
	Pattern string `json:"pattern,omitempty"`
}

// ListUnadoptedRepos lists unadopted repositories
func (c *AdminService) ListUnadoptedRepos(ctx context.Context, opt ListUnadoptedReposOptions) ([]string, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/unadopted")
	query := opt.getURLQuery()
	if opt.Pattern != "" {
		query.Add("pattern", opt.Pattern)
	}
	link.RawQuery = query.Encode()

	repos := make([]string, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &repos)
	return repos, resp, err
}

// AdoptUnadoptedRepo adopts an unadopted repository
func (c *AdminService) AdoptUnadoptedRepo(ctx context.Context, owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST",
		fmt.Sprintf("/admin/unadopted/%s/%s", owner, repo),
		jsonHeader, nil)
}

// DeleteUnadoptedRepo deletes an unadopted repository
func (c *AdminService) DeleteUnadoptedRepo(ctx context.Context, owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/admin/unadopted/%s/%s", owner, repo),
		jsonHeader, nil)
}

// AdminListOrgsOptions options for listing admin's organizations
type AdminListOrgsOptions struct {
	ListOptions
}

// ListOrgs lists all orgs
func (c *AdminService) ListOrgs(ctx context.Context, opt AdminListOrgsOptions) ([]*Organization, *Response, error) {
	opt.setDefaults()
	orgs := make([]*Organization, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/admin/orgs?%s", opt.getURLQuery().Encode()), nil, nil, &orgs)
	return orgs, resp, err
}

// CreateOrg create an organization
func (c *AdminService) CreateOrg(ctx context.Context, user string, opt CreateOrgOption) (*Organization, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	org := new(Organization)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/admin/users/%s/orgs", user), jsonHeader, bytes.NewReader(body), org)
	return org, resp, err
}
