// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
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
	"time"
)

// Hook a hook is a web hook when one repository changed
type Hook struct {
	ID                  int64             `json:"id"`
	Type                string            `json:"type"`
	URL                 string            `json:"-"`
	BranchFilter        string            `json:"branch_filter"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	AuthorizationHeader string            `json:"authorization_header"`
	Active              bool              `json:"active"`
	Updated             time.Time         `json:"updated_at"`
	Created             time.Time         `json:"created_at"`
}

// HookType represent all webhook types gitea currently offer
type HookType string

const (
	// HookTypeDingtalk webhook that dingtalk understand
	HookTypeDingtalk HookType = "dingtalk"
	// HookTypeDiscord webhook that discord understand
	HookTypeDiscord HookType = "discord"
	// HookTypeGitea webhook that gitea understand
	HookTypeGitea HookType = "gitea"
	// HookTypeGogs webhook that gogs understand
	HookTypeGogs HookType = "gogs"
	// HookTypeMsteams webhook that msteams understand
	HookTypeMsteams HookType = "msteams"
	// HookTypeSlack webhook that slack understand
	HookTypeSlack HookType = "slack"
	// HookTypeTelegram webhook that telegram understand
	HookTypeTelegram HookType = "telegram"
	// HookTypeFeishu webhook that feishu understand
	HookTypeFeishu HookType = "feishu"
)

// ListHooksOptions options for listing hooks
type ListHooksOptions struct {
	ListOptions
}

// ListOrgHooks list all the hooks of one organization
func (c *HooksService) ListOrgHooks(ctx context.Context, org string, opt ListHooksOptions) ([]*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/orgs/%s/hooks?%s", org, opt.getURLQuery().Encode()), nil, nil, &hooks)
	return hooks, resp, err
}

// ListMyHooks list all the hooks of the authenticated user
func (c *HooksService) ListMyHooks(ctx context.Context, opt ListHooksOptions) ([]*Hook, *Response, error) {
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/hooks?%s", opt.getURLQuery().Encode()), nil, nil, &hooks)
	return hooks, resp, err
}

// ListRepoHooks list all the hooks of one repository
func (c *HooksService) ListRepoHooks(ctx context.Context, user, repo string, opt ListHooksOptions) ([]*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/hooks?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &hooks)
	return hooks, resp, err
}

// GetOrgHook get a hook of an organization
func (c *HooksService) GetOrgHook(ctx context.Context, org string, id int64) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil, h)
	return h, resp, err
}

// GetMyHook get a hook of the authenticated user
func (c *HooksService) GetMyHook(ctx context.Context, id int64) (*Hook, *Response, error) {
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/hooks/%d", id), nil, nil, h)
	return h, resp, err
}

// GetRepoHook get a hook of a repository
func (c *HooksService) GetRepoHook(ctx context.Context, user, repo string, id int64) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil, h)
	return h, resp, err
}

// CreateHookOption options when create a hook
type CreateHookOption struct {
	Type                HookType          `json:"type"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	BranchFilter        string            `json:"branch_filter"`
	Active              bool              `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
}

// Validate the CreateHookOption struct
func (opt CreateHookOption) Validate() error {
	if len(opt.Type) == 0 {
		return fmt.Errorf("hook type needed")
	}
	return nil
}

// CreateOrgHook create one hook for an organization, with options
func (c *HooksService) CreateOrgHook(ctx context.Context, org string, opt CreateHookOption) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/orgs/%s/hooks", org), jsonHeader, bytes.NewReader(body), h)
	return h, resp, err
}

// CreateMyHook create one hook for the authenticated user, with options
func (c *HooksService) CreateMyHook(ctx context.Context, opt CreateHookOption) (*Hook, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "POST", "/user/hooks", jsonHeader, bytes.NewReader(body), h)
	return h, resp, err
}

// CreateRepoHook create one hook for a repository, with options
func (c *HooksService) CreateRepoHook(ctx context.Context, user, repo string, opt CreateHookOption) (*Hook, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	h := new(Hook)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/hooks", user, repo), jsonHeader, bytes.NewReader(body), h)
	return h, resp, err
}

// EditHookOption options when modify one hook
type EditHookOption struct {
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	BranchFilter        string            `json:"branch_filter"`
	Active              *bool             `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
}

// EditOrgHook modify one hook of an organization, with hook id and options
func (c *HooksService) EditOrgHook(ctx context.Context, org string, id int64, opt EditHookOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PATCH", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), jsonHeader, bytes.NewReader(body))
}

// EditMyHook modify one hook of the authenticated user, with hook id and options
func (c *HooksService) EditMyHook(ctx context.Context, id int64, opt EditHookOption) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PATCH", fmt.Sprintf("/user/hooks/%d", id), jsonHeader, bytes.NewReader(body))
}

// EditRepoHook modify one hook of a repository, with hook id and options
func (c *HooksService) EditRepoHook(ctx context.Context, user, repo string, id int64, opt EditHookOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), jsonHeader, bytes.NewReader(body))
}

// DeleteOrgHook delete one hook from an organization, with hook id
func (c *HooksService) DeleteOrgHook(ctx context.Context, org string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/orgs/%s/hooks/%d", org, id), nil, nil)
}

// DeleteMyHook delete one hook from the authenticated user, with hook id
func (c *HooksService) DeleteMyHook(ctx context.Context, id int64) (*Response, error) {
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/hooks/%d", id), nil, nil)
}

// DeleteRepoHook delete one hook from a repository, with hook id
func (c *HooksService) DeleteRepoHook(ctx context.Context, user, repo string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", user, repo, id), nil, nil)
}

// TestWebhook tests a webhook.
func (c *HooksService) TestWebhook(ctx context.Context, owner, repo string, hookID int64, ref string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	opt := map[string]string{"ref": ref}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/hooks/%d/tests", owner, repo, hookID),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// ListGlobalHooksOptions options for listing global hooks.
type ListGlobalHooksOptions struct {
	ListOptions
	// Type of hooks to list: system, default, or all
	Type string `json:"type,omitempty"`
}

// Deprecated: use ListGlobalHooksOptions.
type ListAdminHooksOptions = ListGlobalHooksOptions

// ListGlobalHooks lists all global webhooks.
func (c *HooksService) ListGlobalHooks(ctx context.Context, opt ListGlobalHooksOptions) ([]*Hook, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/hooks")
	query := opt.getURLQuery()
	if opt.Type != "" {
		query.Add("type", opt.Type)
	}
	link.RawQuery = query.Encode()

	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &hooks)
	return hooks, resp, err
}

// CreateGlobalHook creates a global webhook.
func (c *HooksService) CreateGlobalHook(ctx context.Context, opt CreateHookOption) (*Hook, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	hook := new(Hook)
	resp, err := c.getParsedResponse(ctx, "POST", "/admin/hooks", jsonHeader, bytes.NewReader(body), hook)
	return hook, resp, err
}

// GetGlobalHook gets a global webhook by ID.
func (c *HooksService) GetGlobalHook(ctx context.Context, id int64) (*Hook, *Response, error) {
	hook := new(Hook)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, nil, hook)
	return hook, resp, err
}

// EditGlobalHook edits a global webhook.
func (c *HooksService) EditGlobalHook(ctx context.Context, id int64, opt EditHookOption) (*Hook, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	hook := new(Hook)
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, bytes.NewReader(body), hook)
	return hook, resp, err
}

// DeleteGlobalHook deletes a global webhook.
func (c *HooksService) DeleteGlobalHook(ctx context.Context, id int64) (*Response, error) {
	status, resp, err := c.getStatusCode(ctx, "DELETE", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
