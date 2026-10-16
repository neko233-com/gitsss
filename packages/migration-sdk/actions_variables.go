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

// ActionsVariable represents an Actions variable.
type ActionsVariable struct {
	OwnerID     int64  `json:"owner_id"`
	RepoID      int64  `json:"repo_id"`
	Name        string `json:"name"`
	Data        string `json:"data"`
	Description string `json:"description"`
}

// Deprecated: use ActionsVariable instead.
type ActionVariable = ActionsVariable

// CreateActionsVariableOption is used to create an Actions variable.
type CreateActionsVariableOption struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Deprecated: use CreateActionsVariableOption instead.
type CreateActionVariableOption = CreateActionsVariableOption

// Validate checks whether the variable create payload is valid.
func (opt CreateActionsVariableOption) Validate() error {
	if len(opt.Value) == 0 {
		return errors.New("empty Value field")
	}
	return nil
}

// UpdateActionsVariableOption is used to update an Actions variable.
type UpdateActionsVariableOption struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Deprecated: use UpdateActionsVariableOption instead.
type UpdateActionVariableOption = UpdateActionsVariableOption

// Validate checks whether the variable update payload is valid.
func (opt UpdateActionsVariableOption) Validate() error {
	if len(opt.Value) == 0 {
		return errors.New("empty Value field")
	}
	return nil
}

// CreateActionsVariable represents body for creating a action variable.
type CreateRepoActionsVariable struct {
	Value string `json:"value"`
}

// PutActionsVariable represents body for updating a action variable.
type PutRepoActionsVariable struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// RepoActionsVariable represents an Actions repository variable.
type RepoActionsVariable struct {
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
	Name    string `json:"name"`
	Value   string `json:"data"`
}

// Deprecated: use RepoActionsVariable instead.
type RepoActionVariable = RepoActionsVariable

// ListRepoActionsVariableOption lists RepoActionsVariable options
type ListRepoActionsVariableOption struct {
	ListOptions
}

// Deprecated: use ListRepoActionsVariableOption instead.
type ListRepoActionVariableOption = ListRepoActionsVariableOption

// ListRepoActionVariable lists a repository's action variables
func (c *ActionsService) ListRepoVariables(ctx context.Context, user, repo string, opt ListRepoActionsVariableOption) ([]*RepoActionsVariable, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*RepoActionsVariable, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/variables", user, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetRepoActionVariable returns a repository variable in the Gitea Actions.
// It takes the repository owner, name and the variable name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *ActionsService) GetRepoVariable(ctx context.Context, user, repo, variableName string) (*RepoActionsVariable, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	variable := new(RepoActionsVariable)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), nil, nil, variable)
	return variable, resp, err
}

// CreateRepoActionVariable creates a repository variable in the Gitea Actions.
// It takes the repository owner, name, variable name and the variable value as parameters.
// The function returns the HTTP response and an error, if any.
func (c *ActionsService) CreateRepoVariable(ctx context.Context, user, repo, variableName, value string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	create := CreateRepoActionsVariable{
		Value: value,
	}

	body, err := json.Marshal(&create)
	if err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), jsonHeader, bytes.NewReader(body))
}

// UpdateRepoActionVariable updates a repository variable in the Gitea Actions.
// It takes the repository owner, name, variable name and the variable value as parameters.
// The function returns the HTTP response and an error, if any.
func (c *ActionsService) UpdateRepoVariable(ctx context.Context, user, repo, variableName, value string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	update := PutRepoActionsVariable{
		Value: value,
		Name:  variableName,
	}

	body, err := json.Marshal(&update)
	if err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), jsonHeader, bytes.NewReader(body))
}

// DeleteRepoActionVariable deletes a repository variable in the Gitea Actions.
// It takes the repository owner, name and the variable name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *ActionsService) DeleteRepoVariable(ctx context.Context, user, reponame, variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &reponame); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, reponame, variableName), nil, nil)
}

// ListOrgActionsVariableOption lists ActionVariable options
type ListOrgActionsVariableOption struct {
	ListOptions
}

// Deprecated: use ListOrgActionsVariableOption instead.
type ListOrgActionVariableOption = ListOrgActionsVariableOption

// ListOrgActionVariable lists an organization's action variables
func (c *ActionsService) ListOrgVariables(ctx context.Context, org string, opt ListOrgActionsVariableOption) ([]*ActionsVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*ActionsVariable, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/variables", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetOrgActionVariable gets a single organization's action variable by name
func (c *ActionsService) GetOrgVariable(ctx context.Context, org, name string) (*ActionsVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, nil, err
	}
	var variable ActionsVariable
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name),
		jsonHeader, nil, &variable)
	if err != nil {
		return nil, resp, err
	}
	return &variable, resp, nil
}

// CreateOrgActionVariable creates a variable for the specified organization in the Gitea Actions.
func (c *ActionsService) CreateOrgVariable(ctx context.Context, org, name string, opt CreateActionsVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), jsonHeader, bytes.NewReader(body))
}

// UpdateOrgActionVariable updates a variable for the specified organization in the Gitea Actions.
func (c *ActionsService) UpdateOrgVariable(ctx context.Context, org, name string, opt UpdateActionsVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), jsonHeader, bytes.NewReader(body))
}

// DeleteOrgActionVariable deletes an organization's Actions variable.
func (c *ActionsService) DeleteOrgVariable(ctx context.Context, org, name string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), nil, nil)
}

// ListUserActionVariable lists user-scope Actions variables.
func (c *ActionsService) ListUserVariables(ctx context.Context, opt ListOptions) ([]*ActionsVariable, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*ActionsVariable, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/actions/variables?%s", opt.getURLQuery().Encode()), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetUserActionVariable gets one user-scope Actions variable.
func (c *ActionsService) GetUserVariable(ctx context.Context, variableName string) (*ActionsVariable, *Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, nil, err
	}
	variable := new(ActionsVariable)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, nil, variable)
	return variable, resp, err
}

// CreateUserActionVariable creates one user-scope Actions variable.
func (c *ActionsService) CreateUserVariable(ctx context.Context, variableName string, opt CreateActionsVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
}

// UpdateUserActionVariable updates one user-scope Actions variable.
func (c *ActionsService) UpdateUserVariable(ctx context.Context, variableName string, opt UpdateActionsVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
}

// DeleteUserActionVariable deletes one user-scope Actions variable.
func (c *ActionsService) DeleteUserVariable(ctx context.Context, variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_22_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/actions/variables/%s", variableName), nil, nil)
}
