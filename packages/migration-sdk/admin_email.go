// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"net/url"
)

// ListAdminEmailsOptions options for listing all emails
type ListAdminEmailsOptions struct {
	ListOptions
}

// ListEmails lists all email addresses
func (c *AdminService) ListEmails(ctx context.Context, opt ListAdminEmailsOptions) ([]*Email, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/emails")
	link.RawQuery = opt.getURLQuery().Encode()

	emails := make([]*Email, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &emails)
	return emails, resp, err
}

// SearchAdminEmailsOptions options for searching emails
type SearchAdminEmailsOptions struct {
	ListOptions
	Query string `json:"q,omitempty"`
}

// SearchEmails searches email addresses
func (c *AdminService) SearchEmails(ctx context.Context, opt SearchAdminEmailsOptions) ([]*Email, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/emails/search")
	query := opt.getURLQuery()
	if opt.Query != "" {
		query.Add("q", opt.Query)
	}
	link.RawQuery = query.Encode()

	emails := make([]*Email, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &emails)
	return emails, resp, err
}
