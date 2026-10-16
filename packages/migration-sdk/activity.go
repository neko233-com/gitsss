// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Activity represents a user or organization activity
type Activity struct {
	ID        int64       `json:"id"`
	ActUserID int64       `json:"act_user_id"`
	ActUser   *User       `json:"act_user"`
	OpType    string      `json:"op_type"`
	Content   string      `json:"content"`
	RepoID    int64       `json:"repo_id"`
	Repo      *Repository `json:"repo"`
	CommentID int64       `json:"comment_id"`
	Comment   *Comment    `json:"comment"`
	RefName   string      `json:"ref_name"`
	IsPrivate bool        `json:"is_private"`
	UserID    int64       `json:"user_id"`
	Created   time.Time   `json:"created"`
}

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     int64 `json:"timestamp"`
	Contributions int64 `json:"contributions"`
}

// GetUserHeatmap gets a user's heatmap data.
func (c *ActivityService) GetUserHeatmap(ctx context.Context, username string) ([]*UserHeatmapData, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, nil, err
	}

	heatmap := make([]*UserHeatmapData, 0, 365)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/users/%s/heatmap", username),
		jsonHeader, nil, &heatmap)
	return heatmap, resp, err
}

// ListUserActivityFeedsOptions options for listing user activity feeds
type ListUserActivityFeedsOptions struct {
	ListOptions
	OnlyPerformedBy bool   `json:"only-performed-by,omitempty"`
	Date            string `json:"date,omitempty"`
}

// ListUserActivityFeeds lists a user's activity feeds
func (c *ActivityService) ListUserFeeds(ctx context.Context, username string, opt ListUserActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/users/%s/activities/feeds", username))
	query := opt.getURLQuery()
	if opt.OnlyPerformedBy {
		query.Add("only-performed-by", "true")
	}
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}

// ListRepoActivityFeedsOptions options for listing repository activity feeds
type ListRepoActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date"` // the date of the activities to be found (format: YYYY-MM-DD)
}

// ListRepoActivityFeeds lists activity feeds for a repository
func (c *ActivityService) ListRepoFeeds(ctx context.Context, owner, repo string, opt ListRepoActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/activities/feeds", owner, repo))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	feeds := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &feeds)
	return feeds, resp, err
}

// ListOrgActivityFeedsOptions options for listing organization activity feeds
type ListOrgActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date,omitempty"`
}

// ListOrgActivityFeeds lists the organization's activity feeds
func (c *ActivityService) ListOrgFeeds(ctx context.Context, org string, opt ListOrgActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/activities/feeds", org))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}

// ListTeamActivityFeedsOptions options for listing team activity feeds
type ListTeamActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date,omitempty"`
}

// ListTeamActivityFeeds lists the team's activity feeds
func (c *ActivityService) ListTeamFeeds(ctx context.Context, teamID int64, opt ListTeamActivityFeedsOptions) ([]*Activity, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/teams/%d/activities/feeds", teamID))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}
