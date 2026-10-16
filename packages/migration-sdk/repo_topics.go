// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// ListRepoTopicsOptions options for listing repo's topics
type ListRepoTopicsOptions struct {
	ListOptions
}

// topicsList represents a list of repo's topics
type topicsList struct {
	Topics []string `json:"topics"`
}

// ListRepoTopics list all repository's topics
func (c *RepositoriesService) ListRepoTopics(ctx context.Context, user, repo string, opt ListRepoTopicsOptions) ([]string, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	list := new(topicsList)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/topics?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, list)
	if err != nil {
		return nil, resp, err
	}
	return list.Topics, resp, nil
}

// SetRepoTopics replaces the list of repo's topics
func (c *RepositoriesService) SetRepoTopics(ctx context.Context, user, repo string, list []string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	l := topicsList{Topics: list}
	body, err := json.Marshal(&l)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/topics", user, repo), jsonHeader, bytes.NewReader(body))
}

// AddRepoTopic adds a topic to a repo's topics list
func (c *RepositoriesService) AddRepoTopic(ctx context.Context, user, repo, topic string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &topic); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/topics/%s", user, repo, topic), nil, nil)
}

// DeleteRepoTopic deletes a topic from repo's topics list
func (c *RepositoriesService) DeleteRepoTopic(ctx context.Context, user, repo, topic string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &topic); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/topics/%s", user, repo, topic), nil, nil)
}

// TopicSearchOptions options for searching topics
type TopicSearchOptions struct {
	ListOptions
	Query string `json:"q"` // query string
}

// TopicSearchResult represents a topic search result
type TopicSearchResult struct {
	Topics []*TopicResponse `json:"topics"`
}

// TopicResponse represents a topic
type TopicResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"topic_name"`
	RepoCount int       `json:"repo_count"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// SearchTopics searches for topics
func (c *RepositoriesService) SearchTopics(ctx context.Context, opt TopicSearchOptions) (*TopicSearchResult, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/topics/search")
	query := opt.getURLQuery()
	if opt.Query != "" {
		query.Add("q", opt.Query)
	}
	link.RawQuery = query.Encode()

	result := new(TopicSearchResult)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &result)
	return result, resp, err
}
