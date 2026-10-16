// Copyright 2026 The Gitea Authors. All rights reserved.
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
)

// ListIssueAttachments lists all attachments for an issue.
func (c *IssuesService) ListIssueAttachments(ctx context.Context, owner, repo string, index int64) ([]*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	attachments := make([]*Attachment, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, index),
		nil, nil, &attachments)
	return attachments, resp, err
}

// GetIssueAttachment gets an issue attachment.
func (c *IssuesService) GetIssueAttachment(ctx context.Context, owner, repo string, index, attachmentID int64) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, index, attachmentID),
		nil, nil, attachment)
	return attachment, resp, err
}

// CreateIssueAttachment uploads an attachment for an issue.
func (c *IssuesService) CreateIssueAttachment(ctx context.Context, owner, repo string, index int64, file io.Reader, filename string) (*Attachment, *Response, error) {
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

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, index))
	link.RawQuery = url.Values{"name": []string{filename}}.Encode()

	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "POST", link.String(), http.Header{"Content-Type": []string{writer.FormDataContentType()}}, body, attachment)
	return attachment, resp, err
}

// EditIssueAttachment updates an issue attachment.
func (c *IssuesService) EditIssueAttachment(ctx context.Context, owner, repo string, index, attachmentID int64, form EditAttachmentOptions) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&form)
	if err != nil {
		return nil, nil, err
	}
	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "PATCH",
		fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, index, attachmentID),
		jsonHeader, bytes.NewReader(body), attachment)
	return attachment, resp, err
}

// DeleteIssueAttachment deletes an issue attachment.
func (c *IssuesService) DeleteIssueAttachment(ctx context.Context, owner, repo string, index, attachmentID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/assets/%d", owner, repo, index, attachmentID), nil, nil)
}
