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
)

// RegistrationToken is returned when creating an Actions runner registration token.
type RegistrationToken struct {
	Token string `json:"token"`
}

// ListActionsRunnersOptions controls runner listing requests.
type ListActionsRunnersOptions struct {
	ListOptions
	Disabled *bool
}

// Deprecated: use ListActionsRunnersOptions instead.
type ListActionRunnersOptions = ListActionsRunnersOptions

// QueryEncode turns the runner list options into a query string.
func (opt *ListActionsRunnersOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Disabled != nil {
		query.Add("disabled", fmt.Sprintf("%t", *opt.Disabled))
	}
	return query.Encode()
}

// ActionsRunnerLabel represents a runner label.
type ActionsRunnerLabel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Deprecated: use ActionsRunnerLabel instead.
type ActionRunnerLabel = ActionsRunnerLabel

// ActionsRunner represents an Actions runner.
type ActionsRunner struct {
	ID        int64                 `json:"id"`
	Name      string                `json:"name"`
	Status    string                `json:"status"`
	Busy      bool                  `json:"busy"`
	Disabled  bool                  `json:"disabled"`
	Ephemeral bool                  `json:"ephemeral"`
	Labels    []*ActionsRunnerLabel `json:"labels"`
}

// Deprecated: use ActionsRunner instead.
type ActionRunner = ActionsRunner

// EditActionsRunnerOption contains editable runner fields.
type EditActionsRunnerOption struct {
	Disabled *bool `json:"disabled"`
}

// Deprecated: use EditActionsRunnerOption instead.
type EditActionRunnerOption = EditActionsRunnerOption

// Validate checks whether the runner update payload is valid.
func (opt EditActionsRunnerOption) Validate() error {
	if opt.Disabled == nil {
		return errors.New("nil Disabled field")
	}
	return nil
}

// ActionsRunnersResponse contains a page of runners.
type ActionsRunnersResponse struct {
	Runners    []*ActionsRunner `json:"runners"`
	TotalCount int64            `json:"total_count"`
}

// Deprecated: use ActionsRunnersResponse instead.
type ActionRunnersResponse = ActionsRunnersResponse

func (c *ActionsService) createActionRegistrationToken(ctx context.Context, path string) (*RegistrationToken, *Response, error) {
	token := new(RegistrationToken)
	resp, err := c.getParsedResponse(ctx, "POST", path, nil, nil, token)
	return token, resp, err
}

func (c *ActionsService) listActionRunners(ctx context.Context, path string, opt ListActionsRunnersOptions) (*ActionsRunnersResponse, *Response, error) {
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionsRunnersResponse)
	response, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

func (c *ActionsService) getActionRunner(ctx context.Context, path string) (*ActionsRunner, *Response, error) {
	runner := new(ActionsRunner)
	resp, err := c.getParsedResponse(ctx, "GET", path, jsonHeader, nil, runner)
	return runner, resp, err
}

func (c *ActionsService) updateActionRunner(ctx context.Context, path string, opt EditActionsRunnerOption) (*ActionsRunner, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	runner := new(ActionsRunner)
	resp, err := c.getParsedResponse(ctx, "PATCH", path, jsonHeader, bytes.NewReader(body), runner)
	return runner, resp, err
}

// CreateRepoActionRunnerRegistrationToken creates a repository-scope runner registration token.
func (c *ActionsService) CreateRepoRunnerRegistrationToken(ctx context.Context, owner, repo string) (*RegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(ctx, fmt.Sprintf("/repos/%s/%s/actions/runners/registration-token", owner, repo))
}

// ListRepoActionRunners lists repository-scope Actions runners.
func (c *ActionsService) ListRepoRunners(ctx context.Context, owner, repo string, opt ListActionsRunnersOptions) (*ActionsRunnersResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(ctx, fmt.Sprintf("/repos/%s/%s/actions/runners", owner, repo), opt)
}

// GetRepoActionRunner gets one repository-scope Actions runner.
func (c *ActionsService) GetRepoRunner(ctx context.Context, owner, repo string, runnerID int64) (*ActionsRunner, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(ctx, fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID))
}

// DeleteRepoActionRunner deletes one repository-scope Actions runner.
func (c *ActionsService) DeleteRepoRunner(ctx context.Context, owner, repo string, runnerID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID), nil, nil)
}

// UpdateRepoActionRunner updates one repository-scope Actions runner.
func (c *ActionsService) UpdateRepoRunner(ctx context.Context, owner, repo string, runnerID int64, opt EditActionsRunnerOption) (*ActionsRunner, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(ctx, fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID), opt)
}

// CreateOrgActionRunnerRegistrationToken creates an organization runner registration token.
func (c *ActionsService) CreateOrgRunnerRegistrationToken(ctx context.Context, org string) (*RegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(ctx, fmt.Sprintf("/orgs/%s/actions/runners/registration-token", org))
}

// ListOrgActionRunners lists organization-scoped Actions runners.
func (c *ActionsService) ListOrgRunners(ctx context.Context, org string, opt ListActionsRunnersOptions) (*ActionsRunnersResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(ctx, fmt.Sprintf("/orgs/%s/actions/runners", org), opt)
}

// GetOrgActionRunner gets one organization-scoped Actions runner.
func (c *ActionsService) GetOrgRunner(ctx context.Context, org string, runnerID int64) (*ActionsRunner, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(ctx, fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID))
}

// DeleteOrgActionRunner deletes one organization-scoped Actions runner.
func (c *ActionsService) DeleteOrgRunner(ctx context.Context, org string, runnerID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID), nil, nil)
}

// UpdateOrgActionRunner updates one organization-scoped Actions runner.
func (c *ActionsService) UpdateOrgRunner(ctx context.Context, org string, runnerID int64, opt EditActionsRunnerOption) (*ActionsRunner, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(ctx, fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID), opt)
}

// ListGlobalRunners lists all global Actions runners.
func (c *ActionsService) ListGlobalRunners(ctx context.Context, opt ListActionsRunnersOptions) (*ActionsRunnersResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(ctx, "/admin/actions/runners", opt)
}

// GetGlobalRunner gets one global Actions runner.
func (c *ActionsService) GetGlobalRunner(ctx context.Context, runnerID int64) (*ActionsRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(ctx, fmt.Sprintf("/admin/actions/runners/%d", runnerID))
}

// DeleteGlobalRunner deletes one global Actions runner.
func (c *ActionsService) DeleteGlobalRunner(ctx context.Context, runnerID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/admin/actions/runners/%d", runnerID), nil, nil)
}

// UpdateGlobalRunner updates one global Actions runner.
func (c *ActionsService) UpdateGlobalRunner(ctx context.Context, runnerID int64, opt EditActionsRunnerOption) (*ActionsRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(ctx, fmt.Sprintf("/admin/actions/runners/%d", runnerID), opt)
}

// CreateGlobalRunnerRegistrationToken creates a global runner registration token.
func (c *ActionsService) CreateGlobalRunnerRegistrationToken(ctx context.Context) (*RegistrationToken, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(ctx, "/admin/actions/runners/registration-token")
}

// CreateUserActionRunnerRegistrationToken creates a user-scope runner registration token.
func (c *ActionsService) CreateUserRunnerRegistrationToken(ctx context.Context) (*RegistrationToken, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(ctx, "/user/actions/runners/registration-token")
}

// ListUserActionRunners lists user-scope Actions runners.
func (c *ActionsService) ListUserRunners(ctx context.Context, opt ListActionsRunnersOptions) (*ActionsRunnersResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(ctx, "/user/actions/runners", opt)
}

// GetUserActionRunner gets one user-scope Actions runner.
func (c *ActionsService) GetUserRunner(ctx context.Context, runnerID int64) (*ActionsRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(ctx, fmt.Sprintf("/user/actions/runners/%d", runnerID))
}

// DeleteUserActionRunner deletes one user-scope Actions runner.
func (c *ActionsService) DeleteUserRunner(ctx context.Context, runnerID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/actions/runners/%d", runnerID), nil, nil)
}

// UpdateUserActionRunner updates one user-scope Actions runner.
func (c *ActionsService) UpdateUserRunner(ctx context.Context, runnerID int64, opt EditActionsRunnerOption) (*ActionsRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(ctx, fmt.Sprintf("/user/actions/runners/%d", runnerID), opt)
}
