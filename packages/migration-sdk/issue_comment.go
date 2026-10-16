// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	ID               int64         `json:"id"`
	HTMLURL          string        `json:"html_url"`
	PRURL            string        `json:"pull_request_url"`
	IssueURL         string        `json:"issue_url"`
	Poster           *User         `json:"user"`
	OriginalAuthor   string        `json:"original_author"`
	OriginalAuthorID int64         `json:"original_author_id"`
	Body             string        `json:"body"`
	Created          time.Time     `json:"created_at"`
	Updated          time.Time     `json:"updated_at"`
	Attachments      []*Attachment `json:"assets"`
}

// ListIssueCommentOptions list comment options
type ListIssueCommentOptions struct {
	ListOptions
	Since  time.Time
	Before time.Time
}

// QueryEncode turns options into querystring argument
func (opt *ListIssueCommentOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}
	return query.Encode()
}

// ListIssueComments list comments on an issue.
func (c *IssuesService) ListIssueComments(ctx context.Context, owner, repo string, index int64, opt ListIssueCommentOptions) ([]*Comment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, index))
	link.RawQuery = opt.QueryEncode()
	comments := make([]*Comment, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &comments)
	return comments, resp, err
}

// ListRepoIssueComments list comments for a given repo.
func (c *IssuesService) ListRepoIssueComments(ctx context.Context, owner, repo string, opt ListIssueCommentOptions) ([]*Comment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/comments", owner, repo))
	link.RawQuery = opt.QueryEncode()
	comments := make([]*Comment, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &comments)
	return comments, resp, err
}

// GetIssueComment get a comment for a given repo by id.
func (c *IssuesService) GetIssueComment(ctx context.Context, owner, repo string, id int64) (*Comment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	comment := new(Comment)
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0); err != nil {
		return comment, nil, err
	}
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, id), nil, nil, &comment)
	return comment, resp, err
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	Body string `json:"body"`
}

// Validate the CreateIssueCommentOption struct
func (opt CreateIssueCommentOption) Validate() error {
	if len(opt.Body) == 0 {
		return fmt.Errorf("body is empty")
	}
	return nil
}

// CreateIssueComment create comment on an issue.
func (c *IssuesService) CreateIssueComment(ctx context.Context, owner, repo string, index int64, opt CreateIssueCommentOption) (*Comment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	comment := new(Comment)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, index), jsonHeader, bytes.NewReader(body), comment)
	return comment, resp, err
}

// EditIssueCommentOption options for editing a comment
type EditIssueCommentOption struct {
	Body string `json:"body"`
}

// Validate the EditIssueCommentOption struct
func (opt EditIssueCommentOption) Validate() error {
	if len(opt.Body) == 0 {
		return fmt.Errorf("body is empty")
	}
	return nil
}

// EditIssueComment edits an issue comment.
func (c *IssuesService) EditIssueComment(ctx context.Context, owner, repo string, commentID int64, opt EditIssueCommentOption) (*Comment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	comment := new(Comment)
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, commentID), jsonHeader, bytes.NewReader(body), comment)
	return comment, resp, err
}

// DeleteIssueComment deletes an issue comment.
func (c *IssuesService) DeleteIssueComment(ctx context.Context, owner, repo string, commentID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, commentID), nil, nil)
}

// ListIssueCommentAttachments lists all attachments for a comment
func (c *IssuesService) ListIssueCommentAttachments(ctx context.Context, owner, repo string, commentID int64) ([]*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	attachments := make([]*Attachment, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets", owner, repo, commentID),
		nil, nil, &attachments)
	return attachments, resp, err
}

// CreateIssueCommentAttachment uploads an attachment for a comment.
func (c *IssuesService) CreateIssueCommentAttachment(ctx context.Context, owner, repo string, commentID int64, file io.Reader, filename string) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("attachment", filename)
	if err != nil {
		return nil, nil, err
	}
	if _, err = io.Copy(part, file); err != nil {
		return nil, nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets", owner, repo, commentID))
	link.RawQuery = url.Values{"name": []string{filename}}.Encode()

	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "POST", link.String(), http.Header{"Content-Type": []string{writer.FormDataContentType()}}, body, attachment)
	return attachment, resp, err
}

// GetIssueCommentAttachment gets a comment attachment
func (c *IssuesService) GetIssueCommentAttachment(ctx context.Context, owner, repo string, commentID, attachmentID int64) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, commentID, attachmentID),
		nil, nil, &attachment)
	return attachment, resp, err
}

// EditIssueCommentAttachment updates a comment attachment
func (c *IssuesService) EditIssueCommentAttachment(ctx context.Context, owner, repo string, commentID, attachmentID int64, form EditAttachmentOptions) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&form)
	if err != nil {
		return nil, nil, err
	}
	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "PATCH",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, commentID, attachmentID),
		jsonHeader, bytes.NewReader(body), attachment)
	return attachment, resp, err
}

// DeleteIssueCommentAttachment deletes a comment attachment
func (c *IssuesService) DeleteIssueCommentAttachment(ctx context.Context, owner, repo string, commentID, attachmentID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/comments/%d/assets/%d", owner, repo, commentID, attachmentID), nil, nil)
}

// Comment represents a comment on a commit or issue
type TimelineComment struct {
	ID               int64      `json:"id"`
	HTMLURL          string     `json:"html_url"`
	PRURL            string     `json:"pull_request_url"`
	IssueURL         string     `json:"issue_url"`
	Poster           *User      `json:"user"`
	OriginalAuthor   string     `json:"original_author"`
	OriginalAuthorID int64      `json:"original_author_id"`
	Body             string     `json:"body"`
	Created          time.Time  `json:"created_at"`
	Updated          time.Time  `json:"updated_at"`
	Type             string     `json:"type"`
	Label            []*Label   `json:"label"`
	NewMilestone     *Milestone `json:"milestone"`
	OldMilestone     *Milestone `json:"old_milestone"`
	NewTitle         string     `json:"new_title"`
	OldTitle         string     `json:"old_title"`
}

// ListIssueTimeline list timeline on an issue.
func (c *IssuesService) ListIssueTimeline(ctx context.Context, owner, repo string, index int64, opt ListIssueCommentOptions) ([]*TimelineComment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/timeline", owner, repo, index))
	link.RawQuery = opt.QueryEncode()
	timelineComments := make([]*TimelineComment, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), nil, nil, &timelineComments)
	return timelineComments, resp, err
}
