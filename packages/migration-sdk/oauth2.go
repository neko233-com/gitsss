// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// OAuth2Application represents an OAuth2 application.
type OAuth2Application struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	ClientID           string    `json:"client_id"`
	ClientSecret       string    `json:"client_secret"`
	RedirectURIs       []string  `json:"redirect_uris"`
	ConfidentialClient bool      `json:"confidential_client"`
	Created            time.Time `json:"created"`
}

// Deprecated: use OAuth2Application.
type Oauth2 = OAuth2Application

// ListApplicationsOptions controls OAuth2 application listing requests.
type ListApplicationsOptions struct {
	ListOptions
}

// Deprecated: use ListApplicationsOptions.
type ListOauth2Option = ListApplicationsOptions

// CreateApplicationOption contains the options for creating an application.
type CreateApplicationOption struct {
	Name               string   `json:"name"`
	ConfidentialClient bool     `json:"confidential_client"`
	RedirectURIs       []string `json:"redirect_uris"`
}

// Deprecated: use CreateApplicationOption.
type CreateOauth2Option = CreateApplicationOption

// CreateApplication creates an OAuth2 application and returns a completed OAuth2Application object.
func (c *OAuth2Service) CreateApplication(ctx context.Context, opt CreateApplicationOption) (*OAuth2Application, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	oauth := new(OAuth2Application)
	resp, err := c.getParsedResponse(ctx, "POST", "/user/applications/oauth2", jsonHeader, bytes.NewReader(body), oauth)
	return oauth, resp, err
}

// UpdateApplication updates a specific OAuth2 application by ID and returns a completed OAuth2Application object.
func (c *OAuth2Service) UpdateApplication(ctx context.Context, oauth2id int64, opt CreateApplicationOption) (*OAuth2Application, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	oauth := new(OAuth2Application)
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), jsonHeader, bytes.NewReader(body), oauth)
	return oauth, resp, err
}

// GetApplication returns a specific OAuth2 application by ID.
func (c *OAuth2Service) GetApplication(ctx context.Context, oauth2id int64) (*OAuth2Application, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	oauth2s := &OAuth2Application{}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), nil, nil, &oauth2s)
	return oauth2s, resp, err
}

// ListApplications returns all of your OAuth2 applications.
func (c *OAuth2Service) ListApplications(ctx context.Context, opt ListApplicationsOptions) ([]*OAuth2Application, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	oauth2s := make([]*OAuth2Application, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/applications/oauth2?%s", opt.getURLQuery().Encode()), nil, nil, &oauth2s)
	return oauth2s, resp, err
}

// DeleteApplication deletes an OAuth2 application by ID.
func (c *OAuth2Service) DeleteApplication(ctx context.Context, oauth2id int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, err
	}
	resp, err := c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/applications/oauth2/%d", oauth2id), nil, nil)
	return resp, err
}
