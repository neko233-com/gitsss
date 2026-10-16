// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// FileOptions options for all file APIs
type FileOptions struct {
	// message (optional) for the commit of this file. if not supplied, a default message will be used
	Message string `json:"message"`
	// branch (optional) to base this file from. if not given, the default branch is used
	BranchName string `json:"branch"`
	// new_branch (optional) will make a new branch from `branch` before creating the file
	NewBranchName string `json:"new_branch"`
	// force_push (optional) will do a force-push if the new branch already exists
	ForcePush bool `json:"force_push"`
	// `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
	Author    GitIdentity       `json:"author"`
	Committer GitIdentity       `json:"committer"`
	Dates     CommitDateOptions `json:"dates"`
	// Add a Signed-off-by trailer by the committer at the end of the commit log message.
	Signoff bool `json:"signoff"`
}

// CreateFileOptions options for creating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type CreateFileOptions struct {
	FileOptions
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
}

// DeleteFileOptions options for deleting files (used for other File structs below)
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type DeleteFileOptions struct {
	FileOptions
	// sha is the SHA for the file that already exists
	// required: true
	SHA string `json:"sha"`
}

// UpdateFileOptions options for updating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type UpdateFileOptions struct {
	FileOptions
	// sha is the SHA for the file that already exists
	// required: true
	SHA string `json:"sha"`
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
	// from_path (optional) is the path of the original file which will be moved/renamed to the path in the URL
	FromPath string `json:"from_path"`
}

// FileLinksResponse contains the links for a repo's file
type FileLinksResponse struct {
	Self    *string `json:"self"`
	GitURL  *string `json:"git"`
	HTMLURL *string `json:"html"`
}

// ContentsResponse contains information about a repo's entry's (dir, file, symlink, submodule) metadata and content
type ContentsResponse struct {
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	SHA           string  `json:"sha"`
	LastCommitSha *string `json:"last_commit_sha,omitempty"`
	// swagger:strfmt date-time
	LastCommitterDate *time.Time `json:"last_committer_date,omitempty"`
	// swagger:strfmt date-time
	LastAuthorDate    *time.Time `json:"last_author_date,omitempty"`
	LastCommitMessage *string    `json:"last_commit_message,omitempty"`
	// `type` will be `file`, `dir`, `symlink`, or `submodule`
	Type string `json:"type"`
	Size int64  `json:"size"`
	// `encoding` is populated when `type` is `file`, otherwise null
	Encoding *string `json:"encoding"`
	// `content` is populated when `type` is `file`, otherwise null
	Content *string `json:"content"`
	// `target` is populated when `type` is `symlink`, otherwise null
	Target      *string `json:"target"`
	URL         *string `json:"url"`
	HTMLURL     *string `json:"html_url"`
	GitURL      *string `json:"git_url"`
	DownloadURL *string `json:"download_url"`
	// `submodule_git_url` is populated when `type` is `submodule`, otherwise null
	SubmoduleGitURL *string            `json:"submodule_git_url"`
	Links           *FileLinksResponse `json:"_links"`
	LfsOid          *string            `json:"lfs_oid,omitempty"`
	LfsSize         *int64             `json:"lfs_size,omitempty"`
}

// FileCommitResponse contains information generated from a Git commit for a repo's file.
type FileCommitResponse struct {
	CommitMeta
	HTMLURL   string        `json:"html_url"`
	Author    *CommitUser   `json:"author"`
	Committer *CommitUser   `json:"committer"`
	Parents   []*CommitMeta `json:"parents"`
	Message   string        `json:"message"`
	Tree      *CommitMeta   `json:"tree"`
}

// FileResponse contains information about a repo's file
type FileResponse struct {
	Content      *ContentsResponse          `json:"content"`
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// FileDeleteResponse contains information about a repo's file that was deleted
type FileDeleteResponse struct {
	Content      interface{}                `json:"content"` // to be set to nil
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// GetFile downloads a file of repository, ref can be branch/tag/commit.
// it optional can resolve lfs pointers and server the file instead
// e.g.: ref -> master, filepath -> README.md (no leading slash)
func (c *RepositoriesService) GetFile(ctx context.Context, owner, repo, ref, filepath string, resolveLFS ...bool) ([]byte, *Response, error) {
	reader, resp, err := c.GetFileReader(ctx, owner, repo, ref, filepath, resolveLFS...)
	if reader == nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err2 := io.ReadAll(reader)
	if err2 != nil {
		return nil, resp, err2
	}

	return data, resp, err
}

// GetFileReader return reader for download a file of repository, ref can be branch/tag/commit.
// it optional can resolve lfs pointers and server the file instead
// e.g.: ref -> master, filepath -> README.md (no leading slash)
func (c *RepositoriesService) GetFileReader(ctx context.Context, owner, repo, ref, filepath string, resolveLFS ...bool) (io.ReadCloser, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	// resolve lfs
	if len(resolveLFS) != 0 && resolveLFS[0] {
		if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_17_0); err != nil {
			return nil, nil, err
		}
		return c.getResponseReader(ctx, "GET", fmt.Sprintf("/repos/%s/%s/media/%s?ref=%s", owner, repo, filepath, url.QueryEscape(ref)), nil, nil)
	}

	// normal get
	filepath = pathEscapeSegments(filepath)
	if c.checkServerVersionGreaterThanOrEqual(ctx, version1_14_0) != nil {
		ref = pathEscapeSegments(ref)
		return c.getResponseReader(ctx, "GET", fmt.Sprintf("/repos/%s/%s/raw/%s/%s", owner, repo, ref, filepath), nil, nil)
	}
	return c.getResponseReader(ctx, "GET", fmt.Sprintf("/repos/%s/%s/raw/%s?ref=%s", owner, repo, filepath, url.QueryEscape(ref)), nil, nil)
}

// GetContents get the metadata and contents of a file in a repository
// ref is optional
func (c *RepositoriesService) GetContents(ctx context.Context, owner, repo, ref, filepath string) (*ContentsResponse, *Response, error) {
	data, resp, err := c.getDirOrFileContents(ctx, owner, repo, ref, filepath)
	if err != nil {
		return nil, resp, err
	}
	cr := new(ContentsResponse)
	if json.Unmarshal(data, &cr) != nil {
		return nil, resp, fmt.Errorf("expect file, got directory")
	}
	return cr, resp, err
}

// ListContents gets a list of entries in a dir
// ref is optional
func (c *RepositoriesService) ListContents(ctx context.Context, owner, repo, ref, filepath string) ([]*ContentsResponse, *Response, error) {
	data, resp, err := c.getDirOrFileContents(ctx, owner, repo, ref, filepath)
	if err != nil {
		return nil, resp, err
	}
	crl := make([]*ContentsResponse, 0)
	if json.Unmarshal(data, &crl) != nil {
		return nil, resp, fmt.Errorf("expect directory, got file")
	}
	return crl, resp, err
}

func (c *RepositoriesService) getDirOrFileContents(ctx context.Context, owner, repo, ref, filepath string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(strings.TrimPrefix(filepath, "/"))
	return c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, filepath, url.QueryEscape(ref)), jsonHeader, nil)
}

// CreateFile create a file in a repository
func (c *RepositoriesService) CreateFile(ctx context.Context, owner, repo, filepath string, opt CreateFileOptions) (*FileResponse, *Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(ctx, owner, repo, opt.BranchName); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	fr := new(FileResponse)
	resp, err := c.getParsedResponse(ctx, "POST", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body), fr)
	return fr, resp, err
}

// UpdateFile update a file in a repository
func (c *RepositoriesService) UpdateFile(ctx context.Context, owner, repo, filepath string, opt UpdateFileOptions) (*FileResponse, *Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(ctx, owner, repo, opt.BranchName); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	fr := new(FileResponse)
	resp, err := c.getParsedResponse(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body), fr)
	return fr, resp, err
}

// DeleteFile delete a file from repository
func (c *RepositoriesService) DeleteFile(ctx context.Context, owner, repo, filepath string, opt DeleteFileOptions) (*Response, error) {
	var err error
	if opt.BranchName, err = c.setDefaultBranchForOldVersions(ctx, owner, repo, opt.BranchName); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	filepath = pathEscapeSegments(filepath)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filepath), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != 200 && status != 204 {
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
	return resp, nil
}

func (c *RepositoriesService) setDefaultBranchForOldVersions(ctx context.Context, owner, repo, branch string) (string, error) {
	if len(branch) == 0 {
		// Gitea >= 1.12.0 Use DefaultBranch on "", mimic this for older versions
		if c.checkServerVersionGreaterThanOrEqual(ctx, version1_12_0) != nil {
			r, _, err := c.GetRepo(ctx, owner, repo)
			if err != nil {
				return "", err
			}
			return r.DefaultBranch, nil
		}
	}
	return branch, nil
}

// ChangeFilesOptions options for batch file operations
type ChangeFilesOptions struct {
	Files     []*ChangeFileOperation `json:"files"`
	Message   string                 `json:"message"`
	Branch    string                 `json:"branch,omitempty"`
	NewBranch string                 `json:"new_branch,omitempty"`
	ForcePush bool                   `json:"force_push,omitempty"`
	Author    GitIdentity            `json:"author"`
	Committer GitIdentity            `json:"committer"`
	Dates     CommitDateOptions      `json:"dates"`
	Signoff   bool                   `json:"signoff,omitempty"`
}

// ChangeFileOperation represents a file operation in batch
type ChangeFileOperation struct {
	Operation string `json:"operation"` // create, update, upload, rename, delete
	Path      string `json:"path"`
	Content   string `json:"content"`             // base64 encoded for create/update
	SHA       string `json:"sha,omitempty"`       // required for update/delete
	FromPath  string `json:"from_path,omitempty"` // for rename
}

// ChangeFiles creates, updates, or deletes multiple files
func (c *RepositoriesService) ChangeFiles(ctx context.Context, owner, repo string, opt ChangeFilesOptions) (*FileResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FileResponse)
	resp, err := c.getParsedResponse(ctx, "POST",
		fmt.Sprintf("/repos/%s/%s/contents", owner, repo),
		jsonHeader, bytes.NewReader(body), &result)
	return result, resp, err
}

// GetFilesOptions controls batch file-content lookup requests.
type GetFilesOptions struct {
	Files []string `json:"files"`
}

// Validate checks whether the batch file lookup request is valid.
func (opt GetFilesOptions) Validate() error {
	if len(opt.Files) == 0 {
		return errors.New("empty Files field")
	}
	return nil
}

// GetRepoFileContents fetches metadata and contents for multiple files through the GET endpoint.
// The file list is JSON-encoded in the "body" query parameter; for large file lists prefer
// PostRepoFileContents to avoid URL length limitations.
func (c *RepositoriesService) GetRepoFileContents(ctx context.Context, owner, repo, ref string, opt GetFilesOptions) ([]*ContentsResponse, *Response, error) {
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
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/file-contents", owner, repo))
	query := link.Query()
	if ref != "" {
		query.Add("ref", ref)
	}
	query.Add("body", string(body))
	link.RawQuery = query.Encode()

	contents := make([]*ContentsResponse, 0, len(opt.Files))
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &contents)
	return contents, resp, err
}

// PostRepoFileContents fetches metadata and contents for multiple files through the POST endpoint.
func (c *RepositoriesService) PostRepoFileContents(ctx context.Context, owner, repo, ref string, opt GetFilesOptions) ([]*ContentsResponse, *Response, error) {
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
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/file-contents", owner, repo))
	if ref != "" {
		link.RawQuery = url.Values{"ref": []string{ref}}.Encode()
	}

	contents := make([]*ContentsResponse, 0, len(opt.Files))
	resp, err := c.getParsedResponse(ctx, "POST", link.String(), jsonHeader, bytes.NewReader(body), &contents)
	return contents, resp, err
}

// ContentsExtResponse contains extended information about a repo's contents
type ContentsExtResponse struct {
	DirContents  []*ContentsResponse `json:"dir_contents,omitempty"`
	FileContents *ContentsResponse   `json:"file_contents,omitempty"`
}

// GetContentsExtOptions options for getting extended contents
type GetContentsExtOptions struct {
	// The name of the commit/branch/tag. Default to the repository's default branch
	Ref string `json:"ref,omitempty"`
	// Comma-separated includes options: file_content, lfs_metadata, commit_metadata, commit_message
	Includes string `json:"includes,omitempty"`
}

// GetContentsExt gets extended file metadata and/or content from a repository
// The extended "contents" API, to get file metadata and/or content, or list a directory
func (c *RepositoriesService) GetContentsExt(ctx context.Context, owner, repo, filepath string, opt GetContentsExtOptions) (*ContentsExtResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	// filepath doesn't need escaping since it's already part of the path
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/contents-ext/%s", owner, repo, filepath))
	query := link.Query()

	if opt.Ref != "" {
		query.Add("ref", opt.Ref)
	}
	if opt.Includes != "" {
		query.Add("includes", opt.Includes)
	}

	link.RawQuery = query.Encode()

	result := new(ContentsExtResponse)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, result)
	return result, resp, err
}

// GetEditorConfig gets the EditorConfig definitions of a file in a repository
func (c *RepositoriesService) GetEditorConfig(ctx context.Context, owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/editorconfig/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	return c.getResponse(ctx, "GET", link.String(), nil, nil)
}

// GetRawFileOrLFS gets a file or its LFS object from a repository
// This endpoint resolves LFS pointers and returns actual LFS objects
func (c *RepositoriesService) GetRawFileOrLFS(ctx context.Context, owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/media/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	return c.getResponse(ctx, "GET", link.String(), nil, nil)
}

// GetRawFile gets a file from a repository
// Unlike GetRawFileOrLFS, this does NOT resolve LFS pointers
func (c *RepositoriesService) GetRawFile(ctx context.Context, owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/raw/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	return c.getResponse(ctx, "GET", link.String(), nil, nil)
}
