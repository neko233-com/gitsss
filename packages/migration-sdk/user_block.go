// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListUserBlocksOptions options for listing user blocks
type ListUserBlocksOptions struct {
	ListOptions
}

// ListMyBlocks lists users blocked by the authenticated user
func (c *UsersService) ListMyBlocks(ctx context.Context, opt ListUserBlocksOptions) ([]*User, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/user/blocks")
	link.RawQuery = opt.getURLQuery().Encode()

	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &users)
	return users, resp, err
}

// CheckUserBlock checks if a user is blocked by the authenticated user
func (c *UsersService) CheckUserBlock(ctx context.Context, username string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "GET",
		fmt.Sprintf("/user/blocks/%s", username),
		jsonHeader, nil)
	if err != nil {
		return false, resp, err
	}
	return status == http.StatusNoContent, resp, nil
}

// BlockUser blocks a user
func (c *UsersService) BlockUser(ctx context.Context, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "PUT",
		fmt.Sprintf("/user/blocks/%s", username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnblockUser unblocks a user
func (c *UsersService) UnblockUser(ctx context.Context, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE",
		fmt.Sprintf("/user/blocks/%s", username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
