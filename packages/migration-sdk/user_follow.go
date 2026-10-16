// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
)

// ListFollowersOptions options for listing followers
type ListFollowersOptions struct {
	ListOptions
}

// ListMyFollowers list all the followers of current user
func (c *UsersService) ListMyFollowers(ctx context.Context, opt ListFollowersOptions) ([]*User, *Response, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/followers?%s", opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowers list all the followers of one user
func (c *UsersService) ListFollowers(ctx context.Context, user string, opt ListFollowersOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/followers?%s", user, opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowingOptions options for listing a user's users being followed
type ListFollowingOptions struct {
	ListOptions
}

// ListMyFollowing list all the users current user followed
func (c *UsersService) ListMyFollowing(ctx context.Context, opt ListFollowingOptions) ([]*User, *Response, error) {
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/user/following?%s", opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// ListFollowing list all the users the user followed
func (c *UsersService) ListFollowing(ctx context.Context, user string, opt ListFollowingOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s/following?%s", user, opt.getURLQuery().Encode()), nil, nil, &users)
	return users, resp, err
}

// IsFollowing if current user followed the target
func (c *UsersService) IsFollowing(ctx context.Context, target string) (bool, *Response) {
	if err := escapeValidatePathSegments(&target); err != nil {
		// ToDo return err
		return false, nil
	}
	resp, err := c.doRequestWithStatusHandle(ctx, "GET", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return err == nil, resp
}

// IsUserFollowing if the user followed the target
func (c *UsersService) IsUserFollowing(ctx context.Context, user, target string) (bool, *Response) {
	if err := escapeValidatePathSegments(&user, &target); err != nil {
		// ToDo return err
		return false, nil
	}
	resp, err := c.doRequestWithStatusHandle(ctx, "GET", fmt.Sprintf("/users/%s/following/%s", user, target), nil, nil)
	return err == nil, resp
}

// Follow set current user follow the target
func (c *UsersService) Follow(ctx context.Context, target string) (*Response, error) {
	if err := escapeValidatePathSegments(&target); err != nil {
		return nil, err
	}
	resp, err := c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return resp, err
}

// Unfollow set current user unfollow the target
func (c *UsersService) Unfollow(ctx context.Context, target string) (*Response, error) {
	if err := escapeValidatePathSegments(&target); err != nil {
		return nil, err
	}
	resp, err := c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/user/following/%s", target), nil, nil)
	return resp, err
}
