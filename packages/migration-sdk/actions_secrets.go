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
	"net/url"
	"time"
)

type Secret struct {
	// the secret's name
	Name string `json:"name"`
	// the secret's data
	Data string `json:"data"`
	// the secret's description
	Description string `json:"description"`
	// Date and Time of secret creation
	Created time.Time `json:"created_at"`
}

// CreateOrUpdateSecretOption contains the data for creating or updating an Actions secret.
type CreateOrUpdateSecretOption struct {
	Data        string `json:"data"`
	Description string `json:"description"`
}

// Validate checks whether a secret payload can be sent to the API.
func (opt CreateOrUpdateSecretOption) Validate() error {
	if len(opt.Data) == 0 {
		return errors.New("empty Data field")
	}
	return nil
}

// ListRepoActionsSecretOption list RepoActionSecret options
type ListRepoActionsSecretOption struct {
	ListOptions
}

// Deprecated: use ListRepoActionsSecretOption instead.
type ListRepoActionSecretOption = ListRepoActionsSecretOption

// ListRepoActionSecret list a repository's secrets
func (c *ActionsService) ListRepoSecrets(ctx context.Context, user, repo string, opt ListRepoActionsSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/secrets", user, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateRepoActionSecret creates a secret for the specified repository in the Gitea Actions.
func (c *ActionsService) CreateRepoSecret(ctx context.Context, user, repo, secretName string, opt CreateOrUpdateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &secretName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", user, repo, secretName), jsonHeader, bytes.NewReader(body))
}

// DeleteRepoActionSecret deletes a secret from the Gitea Actions.
// It takes the repository owner, name and the secret name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *ActionsService) DeleteRepoSecret(ctx context.Context, user, repo, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", user, repo, secretName), nil, nil)
}

// ListOrgActionsSecretOption list OrgActionSecret options
type ListOrgActionsSecretOption struct {
	ListOptions
}

// Deprecated: use ListOrgActionsSecretOption instead.
type ListOrgActionSecretOption = ListOrgActionsSecretOption

// ListOrgActionSecret list an organization's secrets
func (c *ActionsService) ListOrgSecrets(ctx context.Context, org string, opt ListOrgActionsSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/secrets", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateOrgActionSecret creates a secret for the specified organization in the Gitea Actions.
func (c *ActionsService) CreateOrgSecret(ctx context.Context, org, secretName string, opt CreateOrUpdateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &secretName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, secretName), jsonHeader, bytes.NewReader(body))
}

// DeleteOrgActionSecret deletes an organization's Actions secret.
func (c *ActionsService) DeleteOrgSecret(ctx context.Context, org, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, secretName), nil, nil)
}

// CreateUserActionSecret creates or updates a user-scope Actions secret.
func (c *ActionsService) CreateUserSecret(ctx context.Context, secretName string, opt CreateOrUpdateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_21_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/user/actions/secrets/%s", secretName), jsonHeader, bytes.NewReader(body))
}

// DeleteUserActionSecret deletes a user-scope Actions secret.
func (c *ActionsService) DeleteUserSecret(ctx context.Context, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_21_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/actions/secrets/%s", secretName), nil, nil)
}
