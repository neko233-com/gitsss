// Copyright 2016 The Gitea Authors. All rights reserved.
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
	"strings"
	"time"
)

// Release represents a repository release
type Release struct {
	ID           int64         `json:"id"`
	TagName      string        `json:"tag_name"`
	Target       string        `json:"target_commitish"`
	Title        string        `json:"name"`
	Note         string        `json:"body"`
	URL          string        `json:"url"`
	HTMLURL      string        `json:"html_url"`
	TarURL       string        `json:"tarball_url"`
	ZipURL       string        `json:"zipball_url"`
	IsDraft      bool          `json:"draft"`
	IsPrerelease bool          `json:"prerelease"`
	CreatedAt    time.Time     `json:"created_at"`
	PublishedAt  time.Time     `json:"published_at"`
	Publisher    *User         `json:"author"`
	Attachments  []*Attachment `json:"assets"`
}

// ListReleasesOptions options for listing repository's releases
type ListReleasesOptions struct {
	ListOptions
	IsDraft      *bool
	IsPreRelease *bool
}

// QueryEncode turns options into querystring argument
func (opt *ListReleasesOptions) QueryEncode() string {
	query := opt.getURLQuery()

	if opt.IsDraft != nil {
		query.Add("draft", fmt.Sprintf("%t", *opt.IsDraft))
	}
	if opt.IsPreRelease != nil {
		query.Add("pre-release", fmt.Sprintf("%t", *opt.IsPreRelease))
	}

	return query.Encode()
}

// ListReleases list releases of a repository
func (c *ReleasesService) ListReleases(ctx context.Context, owner, repo string, opt ListReleasesOptions) ([]*Release, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	releases := make([]*Release, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases?%s", owner, repo, opt.QueryEncode()),
		nil, nil, &releases)
	return releases, resp, err
}

// GetRelease get a release of a repository by id
func (c *ReleasesService) GetRelease(ctx context.Context, owner, repo string, id int64) (*Release, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	r := new(Release)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, id),
		jsonHeader, nil, &r)
	return r, resp, err
}

// GetLatestRelease get the latest release of a repository
func (c *ReleasesService) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	r := new(Release)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases/latest", owner, repo),
		jsonHeader, nil, &r)
	return r, resp, err
}

// GetReleaseByTag get a release of a repository by tag
func (c *ReleasesService) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, *Response, error) {
	if c.checkServerVersionGreaterThanOrEqual(ctx, version1_13_0) != nil {
		return c.fallbackGetReleaseByTag(ctx, owner, repo, tag)
	}
	if err := escapeValidatePathSegments(&owner, &repo, &tag); err != nil {
		return nil, nil, err
	}
	r := new(Release)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, repo, tag),
		nil, nil, &r)
	return r, resp, err
}

// CreateReleaseOption options when creating a release
type CreateReleaseOption struct {
	TagName      string `json:"tag_name"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
}

// Validate the CreateReleaseOption struct
func (opt CreateReleaseOption) Validate() error {
	if len(strings.TrimSpace(opt.Title)) == 0 {
		return fmt.Errorf("title is empty")
	}
	return nil
}

// CreateRelease create a release
func (c *ReleasesService) CreateRelease(ctx context.Context, owner, repo string, opt CreateReleaseOption) (*Release, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(opt)
	if err != nil {
		return nil, nil, err
	}
	r := new(Release)
	resp, err := c.getParsedResponse(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/releases", owner, repo),
		jsonHeader, bytes.NewReader(body), r)
	return r, resp, err
}

// EditReleaseOption options when editing a release
type EditReleaseOption struct {
	TagName      string `json:"tag_name"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      *bool  `json:"draft"`
	IsPrerelease *bool  `json:"prerelease"`
}

// EditRelease edit a release
func (c *ReleasesService) EditRelease(ctx context.Context, owner, repo string, id int64, form EditReleaseOption) (*Release, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(form)
	if err != nil {
		return nil, nil, err
	}
	r := new(Release)
	resp, err := c.getParsedResponse(ctx, "PATCH",
		fmt.Sprintf("/repos/%s/%s/releases/%d", owner, repo, id),
		jsonHeader, bytes.NewReader(body), r)
	return r, resp, err
}

// DeleteRelease delete a release from a repository, keeping its tag
func (c *ReleasesService) DeleteRelease(ctx context.Context, user, repo string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/releases/%d", user, repo, id),
		nil, nil)
}

// DeleteReleaseByTag deletes a release frm a repository by tag
func (c *ReleasesService) DeleteReleaseByTag(ctx context.Context, user, repo, tag string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &tag); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_14_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE",
		fmt.Sprintf("/repos/%s/%s/releases/tags/%s", user, repo, tag),
		nil, nil)
}

// fallbackGetReleaseByTag is fallback for old gitea installations ( < 1.13.0 )
func (c *ReleasesService) fallbackGetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, *Response, error) {
	for i := 1; ; i++ {
		rl, resp, err := c.ListReleases(ctx, owner, repo, ListReleasesOptions{ListOptions: ListOptions{Page: i}})
		if err != nil {
			return nil, resp, err
		}
		if len(rl) == 0 {
			return nil,
				newResponse(&http.Response{StatusCode: 404}),
				fmt.Errorf("release with tag '%s' not found", tag)
		}
		for _, r := range rl {
			if r.TagName == tag {
				return r, resp, nil
			}
		}
	}
}

// Attachment a generic attachment
type Attachment struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	DownloadCount int64     `json:"download_count"`
	Created       time.Time `json:"created_at"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"browser_download_url"`
}

// ListReleaseAttachmentsOptions options for listing release's attachments
type ListReleaseAttachmentsOptions struct {
	ListOptions
}

// ListReleaseAttachments list release's attachments
func (c *ReleasesService) ListReleaseAttachments(ctx context.Context, user, repo string, release int64, opt ListReleaseAttachmentsOptions) ([]*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	attachments := make([]*Attachment, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets?%s", user, repo, release, opt.getURLQuery().Encode()),
		nil, nil, &attachments)
	return attachments, resp, err
}

// GetReleaseAttachment returns the requested attachment
func (c *ReleasesService) GetReleaseAttachment(ctx context.Context, user, repo string, release, id int64) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	a := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, id),
		nil, nil, &a)
	return a, resp, err
}

// CreateReleaseAttachment creates an attachment for the given release
func (c *ReleasesService) CreateReleaseAttachment(ctx context.Context, user, repo string, release int64, file io.Reader, filename string) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	// Write file to body
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

	// Send request
	attachment := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets", user, repo, release),
		http.Header{"Content-Type": []string{writer.FormDataContentType()}}, body, &attachment)
	return attachment, resp, err
}

// EditAttachmentOptions options for editing attachments
type EditAttachmentOptions struct {
	Name string `json:"name"`
}

// EditReleaseAttachment updates the given attachment with the given options
func (c *ReleasesService) EditReleaseAttachment(ctx context.Context, user, repo string, release, attachment int64, form EditAttachmentOptions) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&form)
	if err != nil {
		return nil, nil, err
	}
	attach := new(Attachment)
	resp, err := c.getParsedResponse(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, attachment), jsonHeader, bytes.NewReader(body), attach)
	return attach, resp, err
}

// DeleteReleaseAttachment deletes the given attachment including the uploaded file
func (c *ReleasesService) DeleteReleaseAttachment(ctx context.Context, user, repo string, release, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, id), nil, nil)
}
