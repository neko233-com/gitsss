// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Reference represents a Git reference.
type Reference struct {
	Ref    string     `json:"ref"`
	URL    string     `json:"url"`
	Object *GitObject `json:"object"`
}

// GitObject represents a Git object.
type GitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GetRepoRef gets one exact ref from a repository.
//
// The underlying API returns a filtered list for /git/refs/{ref}, so this
// method resolves the exact ref from that list. It may return HTTP errors from
// the underlying API call, or an error when the server response only contains
// partial matches.
func (c *GitService) GetRepoRef(ctx context.Context, user, repo, ref string) (*Reference, *Response, error) {
	refs, resp, err := c.GetRepoRefs(ctx, user, repo, ref)
	if err != nil {
		return nil, resp, err
	}

	normalizedRef := "refs/" + strings.TrimPrefix(ref, "refs/")
	for _, repoRef := range refs {
		if repoRef == nil || repoRef.Ref != normalizedRef {
			continue
		}
		return repoRef, resp, nil
	}

	return nil, resp, errors.New("no exact match found for this ref")
}

// GetRepoRefs gets the refs from a repository that match a partial or full ref.
func (c *GitService) GetRepoRefs(ctx context.Context, user, repo, ref string) ([]*Reference, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	ref = strings.TrimPrefix(ref, "refs/")
	ref = pathEscapeSegments(ref)

	data, resp, err := c.getResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil)
	if err != nil {
		return nil, resp, err
	}

	// Attempt to unmarshal single returned ref.
	r := new(Reference)
	refErr := json.Unmarshal(data, r)
	if refErr == nil {
		return []*Reference{r}, resp, nil
	}

	// Attempt to unmarshal multiple refs.
	var rs []*Reference
	refsErr := json.Unmarshal(data, &rs)
	if refsErr == nil {
		if len(rs) == 0 {
			return nil, resp, errors.New("unexpected response: an array of refs with length 0")
		}
		return rs, resp, nil
	}

	return nil, resp, fmt.Errorf("unmarshalling failed for both single and multiple refs: %s and %s", refErr, refsErr)
}

// ListAllGitRefs gets all refs from a repository without filtering.
func (c *GitService) ListAllGitRefs(ctx context.Context, owner, repo string) ([]*Reference, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	refs := make([]*Reference, 0, 10)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo), nil, nil, &refs)
	return refs, resp, err
}

// GitBlobResponse represents a git blob
type GitBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	URL      string `json:"url"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
}

// GetBlob get the blob of a repository file
func (c *GitService) GetBlob(ctx context.Context, user, repo, sha string) (*GitBlobResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &sha); err != nil {
		return nil, nil, err
	}
	blob := new(GitBlobResponse)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/repos/%s/%s/git/blobs/%s", user, repo, sha), nil, nil, blob)
	return blob, resp, err
}

// GitNote represents a git note
type GitNote struct {
	Message string  `json:"message"`
	Commit  *Commit `json:"commit"`
}

// Deprecated: use GitNote instead.
type Note = GitNote

// GetRepoNoteOptions options for getting a note
type GetRepoNoteOptions struct {
	// include verification for every commit (disable for speedup, default 'true')
	Verification *bool `json:"verification,omitempty"`
	// include a list of affected files for every commit (disable for speedup, default 'true')
	Files *bool `json:"files,omitempty"`
}

// GetRepoNote gets a note corresponding to a single commit from a repository
func (c *GitService) GetRepoNote(ctx context.Context, owner, repo, sha string, opt GetRepoNoteOptions) (*GitNote, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &sha); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/git/notes/%s", owner, repo, sha))
	query := link.Query()

	if opt.Verification != nil {
		if *opt.Verification {
			query.Add("verification", "true")
		} else {
			query.Add("verification", "false")
		}
	}

	if opt.Files != nil {
		if *opt.Files {
			query.Add("files", "true")
		} else {
			query.Add("files", "false")
		}
	}

	link.RawQuery = query.Encode()

	note := new(GitNote)
	resp, err := c.getParsedResponse(ctx, "GET", link.String(), jsonHeader, nil, &note)
	return note, resp, err
}

// GitEntry represents a git tree
type GitEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitTreeResponse returns a git tree
type GitTreeResponse struct {
	SHA        string     `json:"sha"`
	URL        string     `json:"url"`
	Entries    []GitEntry `json:"tree"`
	Truncated  bool       `json:"truncated"`
	Page       int        `json:"page"`
	TotalCount int        `json:"total_count"`
}

type ListTreeOptions struct {
	ListOptions
	// Ref can be branch/tag/commit. required
	// e.g.: "master"
	Ref string
	// Recursive if true will return the tree in a recursive fashion
	Recursive bool
}

// GetTrees get trees of repository,
func (c *GitService) GetTrees(ctx context.Context, user, repo string, opt ListTreeOptions) (*GitTreeResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &opt.Ref); err != nil {
		return nil, nil, err
	}
	trees := new(GitTreeResponse)
	opt.setDefaults()
	path := fmt.Sprintf("/repos/%s/%s/git/trees/%s?%s", user, repo, opt.Ref, opt.getURLQuery().Encode())

	if opt.Recursive {
		path += "&recursive=1"
	}
	resp, err := c.getParsedResponse(ctx, "GET", path, nil, nil, trees)
	return trees, resp, err
}
