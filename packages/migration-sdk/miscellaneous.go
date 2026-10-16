// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// GitignoreTemplateInfo represents a gitignore template
type GitignoreTemplateInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// LabelTemplate represents a label template
type LabelTemplate struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
}

// LicensesTemplateListEntry represents a license in the list
type LicensesTemplateListEntry struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LicenseTemplateInfo represents a license template
type LicenseTemplateInfo struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Body           string `json:"body"`
	Implementation string `json:"implementation"`
}

// MarkdownOption represents options for rendering markdown
type MarkdownOption struct {
	Text    string `json:"Text"`
	Mode    string `json:"Mode"`
	Context string `json:"Context"`
	Wiki    bool   `json:"Wiki"`
}

// MarkupOption represents options for rendering markup
type MarkupOption struct {
	Text     string `json:"Text"`
	Mode     string `json:"Mode"`
	Context  string `json:"Context"`
	FilePath string `json:"FilePath"`
	Wiki     bool   `json:"Wiki"`
}

// NodeInfo represents nodeinfo about the server
type NodeInfo struct {
	Version           string                 `json:"version"`
	Software          NodeInfoSoftware       `json:"software"`
	Protocols         []string               `json:"protocols"`
	Services          NodeInfoServices       `json:"services"`
	OpenRegistrations bool                   `json:"openRegistrations"`
	Usage             NodeInfoUsage          `json:"usage"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// NodeInfoSoftware represents software information
type NodeInfoSoftware struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Homepage   string `json:"homepage"`
}

// NodeInfoServices represents third party services
type NodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

// NodeInfoUsage represents usage statistics
type NodeInfoUsage struct {
	Users         NodeInfoUsageUsers `json:"users"`
	LocalPosts    int64              `json:"localPosts"`
	LocalComments int64              `json:"localComments"`
}

// NodeInfoUsageUsers represents user statistics
type NodeInfoUsageUsers struct {
	Total          int64 `json:"total"`
	ActiveHalfyear int64 `json:"activeHalfyear"`
	ActiveMonth    int64 `json:"activeMonth"`
}

// ListGitignores lists all gitignore templates.
func (c *TemplatesService) ListGitignores(ctx context.Context) ([]string, *Response, error) {
	templates := make([]string, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET", "/gitignore/templates", jsonHeader, nil, &templates)
	return templates, resp, err
}

// GetGitignore gets information about a gitignore template.
func (c *TemplatesService) GetGitignore(ctx context.Context, name string) (*GitignoreTemplateInfo, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	template := new(GitignoreTemplateInfo)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/gitignore/templates/%s", name),
		jsonHeader, nil, &template)
	return template, resp, err
}

// ListLabels lists all label templates.
func (c *TemplatesService) ListLabels(ctx context.Context) ([]string, *Response, error) {
	templates := make([]string, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET", "/label/templates", jsonHeader, nil, &templates)
	return templates, resp, err
}

// GetLabel gets all labels in a template.
func (c *TemplatesService) GetLabel(ctx context.Context, name string) ([]*LabelTemplate, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	labels := make([]*LabelTemplate, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/label/templates/%s", name),
		jsonHeader, nil, &labels)
	return labels, resp, err
}

// ListLicenses lists all license templates.
func (c *TemplatesService) ListLicenses(ctx context.Context) ([]*LicensesTemplateListEntry, *Response, error) {
	licenses := make([]*LicensesTemplateListEntry, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET", "/licenses", jsonHeader, nil, &licenses)
	return licenses, resp, err
}

// GetLicense gets information about a license template.
func (c *TemplatesService) GetLicense(ctx context.Context, name string) (*LicenseTemplateInfo, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	license := new(LicenseTemplateInfo)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/licenses/%s", name),
		jsonHeader, nil, &license)
	return license, resp, err
}

// RenderMarkdown renders a markdown document as HTML.
func (c *RenderService) RenderMarkdown(ctx context.Context, opt MarkdownOption) (string, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return "", nil, err
	}

	html, resp, err := c.getResponse(ctx, "POST", "/markdown", jsonHeader, bytes.NewReader(body))
	return string(html), resp, err
}

// RenderMarkdownRaw renders raw markdown as HTML.
func (c *RenderService) RenderMarkdownRaw(ctx context.Context, markdown string) (string, *Response, error) {
	html, resp, err := c.getResponse(ctx, "POST", "/markdown/raw",
		map[string][]string{"Content-Type": {"text/plain"}},
		bytes.NewReader([]byte(markdown)))
	if err != nil {
		return "", resp, err
	}
	return string(html), resp, err
}

// RenderMarkup renders a markup document as HTML.
func (c *RenderService) RenderMarkup(ctx context.Context, opt MarkupOption) (string, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return "", nil, err
	}

	html, resp, err := c.getResponse(ctx, "POST", "/markup", jsonHeader, bytes.NewReader(body))
	if err != nil {
		return "", resp, err
	}
	return string(html), resp, err
}

// GetNodeInfo gets the nodeinfo of the Gitea application.
func (c *MetaService) GetNodeInfo(ctx context.Context) (*NodeInfo, *Response, error) {
	nodeInfo := new(NodeInfo)
	resp, err := c.getParsedResponse(ctx, "GET", "/nodeinfo", jsonHeader, nil, &nodeInfo)
	return nodeInfo, resp, err
}

// GetSigningKeyGPG gets the default GPG signing key.
func (c *MetaService) GetSigningKeyGPG(ctx context.Context) (string, *Response, error) {
	key, resp, err := c.getResponse(ctx, "GET", "/signing-key.gpg", nil, nil)
	if err != nil {
		return "", resp, err
	}
	return string(key), resp, err
}

// GetSigningKeySSH gets the default SSH signing key.
func (c *MetaService) GetSigningKeySSH(ctx context.Context) (string, *Response, error) {
	key, resp, err := c.getResponse(ctx, "GET", "/signing-key.pub", nil, nil)
	if err != nil {
		return "", resp, err
	}
	return string(key), resp, err
}
