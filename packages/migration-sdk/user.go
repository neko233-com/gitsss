// Copyright 2014 The Gogs Authors. All rights reserved.
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
	"strconv"
	"time"
)

// User represents a user
type User struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's username
	UserName string `json:"login"`
	// The login_name of non local users (e.g. LDAP / OAuth / SMTP)
	LoginName string `json:"login_name"`
	// The ID of the Authentication Source for non local users.
	SourceID int64 `json:"source_id"`
	// the user's full name
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// URL to the user's profile
	HTMLURL string `json:"html_url"`
	// User locale
	Language string `json:"language"`
	// Is the user an administrator
	IsAdmin bool `json:"is_admin"`
	// Date and Time of last login
	LastLogin time.Time `json:"last_login"`
	// Date and Time of user creation
	Created time.Time `json:"created"`
	// Is user restricted
	Restricted bool `json:"restricted"`
	// Is user active
	IsActive bool `json:"active"`
	// Is user login prohibited
	ProhibitLogin bool `json:"prohibit_login"`
	// the user's location
	Location string `json:"location"`
	// the user's website
	Website string `json:"website"`
	// the user's description
	Description string `json:"description"`
	// User visibility level option
	Visibility VisibleType `json:"visibility"`

	// user counts
	FollowerCount    int `json:"followers_count"`
	FollowingCount   int `json:"following_count"`
	StarredRepoCount int `json:"starred_repos_count"`
}

// GetUserInfo get user info by user's name
func (c *UsersService) GetUserInfo(ctx context.Context, user string) (*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	u := new(User)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/users/%s", user), nil, nil, u)
	return u, resp, err
}

// GetMyUserInfo get user info of current user
func (c *UsersService) GetMyUserInfo(ctx context.Context) (*User, *Response, error) {
	u := new(User)
	resp, err := c.getParsedResponse(ctx, "GET", "/user", nil, nil, u)
	return u, resp, err
}

// GetUserByID returns user by a given user ID
func (c *UsersService) GetUserByID(ctx context.Context, id int64) (*User, *Response, error) {
	if id < 0 {
		return nil, nil, fmt.Errorf("invalid user id %d", id)
	}

	query := make(url.Values)
	query.Add("uid", strconv.FormatInt(id, 10))
	users, resp, err := c.searchUsers(ctx, query.Encode())
	if err != nil {
		return nil, resp, err
	}

	if len(users) == 1 {
		return users[0], resp, err
	}

	return nil, resp, fmt.Errorf("user not found with id %d", id)
}

type searchUsersResponse struct {
	Users []*User `json:"data"`
}

// SearchUsersOption options for SearchUsers
type SearchUsersOption struct {
	ListOptions
	KeyWord string
	UID     int64
}

// QueryEncode turns options into querystring argument
func (opt *SearchUsersOption) QueryEncode() string {
	query := make(url.Values)
	if opt.Page > 0 {
		query.Add("page", fmt.Sprintf("%d", opt.Page))
	}
	if opt.PageSize > 0 {
		query.Add("limit", fmt.Sprintf("%d", opt.PageSize))
	}
	if len(opt.KeyWord) > 0 {
		query.Add("q", opt.KeyWord)
	}
	if opt.UID > 0 {
		query.Add("uid", fmt.Sprintf("%d", opt.UID))
	}
	return query.Encode()
}

func (c *UsersService) searchUsers(ctx context.Context, rawQuery string) ([]*User, *Response, error) {
	link, _ := url.Parse("/users/search")
	link.RawQuery = rawQuery
	userResp := new(searchUsersResponse)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &userResp)
	return userResp.Users, resp, err
}

// SearchUsers finds users by query
func (c *UsersService) SearchUsers(ctx context.Context, opt SearchUsersOption) ([]*User, *Response, error) {
	return c.searchUsers(ctx, opt.QueryEncode())
}

// UpdateUserAvatarOption options for updating user avatar
type UpdateUserAvatarOption struct {
	Image string `json:"image"` // base64 encoded image
}

// UpdateUserAvatar updates the authenticated user's avatar
func (c *UsersService) UpdateUserAvatar(ctx context.Context, opt UpdateUserAvatarOption) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "POST", "/user/avatar",
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// DeleteUserAvatar deletes the authenticated user's avatar
func (c *UsersService) DeleteUserAvatar(ctx context.Context) (*Response, error) {
	status, resp, err := c.getStatusCode(ctx, "DELETE", "/user/avatar", jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UserSettings represents user settings
type UserSettings struct {
	FullName      string `json:"full_name"`
	Website       string `json:"website"`
	Description   string `json:"description"`
	Location      string `json:"location"`
	Language      string `json:"language"`
	Theme         string `json:"theme"`
	DiffViewStyle string `json:"diff_view_style"`
	// Privacy
	HideEmail    bool `json:"hide_email"`
	HideActivity bool `json:"hide_activity"`
}

// UserSettingsOptions represents options to change user settings
type UserSettingsOptions struct {
	FullName      *string `json:"full_name,omitempty"`
	Website       *string `json:"website,omitempty"`
	Description   *string `json:"description,omitempty"`
	Location      *string `json:"location,omitempty"`
	Language      *string `json:"language,omitempty"`
	Theme         *string `json:"theme,omitempty"`
	DiffViewStyle *string `json:"diff_view_style,omitempty"`
	// Privacy
	HideEmail    *bool `json:"hide_email,omitempty"`
	HideActivity *bool `json:"hide_activity,omitempty"`
}

// GetUserSettings returns user settings
func (c *UsersService) GetUserSettings(ctx context.Context) (*UserSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	userConfig := new(UserSettings)
	resp, err := c.getParsedResponse(ctx, "GET", "/user/settings", nil, nil, userConfig)
	return userConfig, resp, err
}

// UpdateUserSettings returns user settings
func (c *UsersService) UpdateUserSettings(ctx context.Context, opt UserSettingsOptions) (*UserSettings, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	userConfig := new(UserSettings)
	resp, err := c.getParsedResponse(ctx, "PATCH", "/user/settings", jsonHeader, bytes.NewReader(body), userConfig)
	return userConfig, resp, err
}
