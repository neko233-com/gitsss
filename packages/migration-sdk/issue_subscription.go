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

// ListIssueSubscribersOptions options for listing issue subscribers
type ListIssueSubscribersOptions struct {
	ListOptions
}

// ListIssueSubscribers get list of users who subscribed on an issue with pagination
func (c *IssuesService) ListIssueSubscribers(ctx context.Context, owner, repo string, index int64, opt ListIssueSubscribersOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions", owner, repo, index))
	opt.setDefaults()
	link.RawQuery = opt.getURLQuery().Encode()
	subscribers := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &subscribers)
	return subscribers, resp, err
}

// AddIssueSubscription Subscribe user to issue
func (c *IssuesService) AddIssueSubscription(ctx context.Context, owner, repo string, index int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &user); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusCreated {
		return resp, nil
	}
	if status == http.StatusOK {
		return resp, fmt.Errorf("already subscribed")
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}

// DeleteIssueSubscription unsubscribe user from issue
func (c *IssuesService) DeleteIssueSubscription(ctx context.Context, owner, repo string, index int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &user); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusCreated {
		return resp, nil
	}
	if status == http.StatusOK {
		return resp, fmt.Errorf("already unsubscribed")
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}

// CheckIssueSubscription check if current user is subscribed to an issue
func (c *IssuesService) CheckIssueSubscription(ctx context.Context, owner, repo string, index int64) (*WatchInfo, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return nil, nil, err
	}
	wi := new(WatchInfo)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/check", owner, repo, index), nil, nil, wi)
	return wi, resp, err
}

// IssueSubscribe subscribe current user to an issue
func (c *IssuesService) IssueSubscribe(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	u, _, err := c.GetMyUserInfo(ctx)
	if err != nil {
		return nil, err
	}
	return c.AddIssueSubscription(ctx, owner, repo, index, u.UserName)
}

// IssueUnSubscribe unsubscribe current user from an issue
func (c *IssuesService) IssueUnSubscribe(ctx context.Context, owner, repo string, index int64) (*Response, error) {
	u, _, err := c.GetMyUserInfo(ctx)
	if err != nil {
		return nil, err
	}
	return c.DeleteIssueSubscription(ctx, owner, repo, index, u.UserName)
}
