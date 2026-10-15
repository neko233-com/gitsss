// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverReadsWikiReviewsAndPackages(t *testing.T) {
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, []any{})
			return
		}
		switch r.URL.Path {
		case "/api/v1/user":
			writeJSON(t, w, User{UserName: "root", IsAdmin: true})
		case "/api/v1/admin/users":
			writeJSON(t, w, []User{{UserName: "alice", IsActive: true}})
		case "/api/v1/admin/orgs":
			writeJSON(t, w, []Organization{})
		case "/api/v1/repos/alice/repo":
			writeJSON(t, w, Repository{
				Owner: &User{UserName: "alice"}, Name: "repo", FullName: "alice/repo", Empty: true,
				CloneURL: source.URL + "/alice/repo.git",
			})
		case "/api/v1/repos/alice/repo/collaborators",
			"/api/v1/repos/alice/repo/labels",
			"/api/v1/repos/alice/repo/milestones",
			"/api/v1/repos/alice/repo/issues":
			writeJSON(t, w, []any{})
		case "/api/v1/repos/alice/repo/pulls":
			writeJSON(t, w, []PullRequest{{
				Number: 2, Title: "Add", State: "open",
				Head: &PullRef{Ref: "topic", Repo: &Repository{FullName: "alice/repo"}},
				Base: &PullRef{Ref: "main"},
			}})
		case "/api/v1/repos/alice/repo/issues/2/comments":
			writeJSON(t, w, []IssueComment{})
		case "/api/v1/repos/alice/repo/pulls/2/reviews":
			writeJSON(t, w, []PullReview{{ID: 9, Body: "ship it", State: "APPROVED", User: &User{UserName: "alice"}}})
		case "/api/v1/repos/alice/repo/pulls/2/reviews/9/comments":
			writeJSON(t, w, []ReviewComment{{Body: "line", Path: "main.go", LineNum: 4}})
		case "/api/v1/repos/alice/repo/releases":
			writeJSON(t, w, []Release{{ID: 3, TagName: "v1", Name: "One"}})
		case "/api/v1/repos/alice/repo/releases/3/assets":
			writeJSON(t, w, []ReleaseAsset{{Name: "notes.txt", Size: 4, DownloadURL: source.URL + "/files/notes.txt"}})
		case "/api/v1/repos/alice/repo/wiki/pages":
			writeJSON(t, w, []WikiPage{{Title: "Home"}})
		case "/api/v1/repos/alice/repo/wiki/page/Home":
			writeJSON(t, w, WikiPage{Title: "Home", ContentBase64: "aGVsbG8="})
		case "/api/v1/packages/alice":
			writeJSON(t, w, []PackageCopy{{
				Type: "generic", Name: "client", Version: "1.0.0",
				Repository: &Repository{FullName: "alice/repo"},
			}})
		case "/api/v1/packages/alice/generic/client/1.0.0/files":
			writeJSON(t, w, []PackageFile{{Name: "client.bin", Size: 4, SHA256: "abcd"}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(source.Close)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "dry-run must not call target", http.StatusInternalServerError)
	}))
	t.Cleanup(target.Close)
	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token",
		TargetURL: target.URL, TargetToken: "target-token",
		Repositories: []string{"alice/repo"},
	})
	require.NoError(t, err)
	plan, err := migrator.Discover(context.Background())
	require.NoError(t, err)
	repo := plan.Repositories[0]
	require.Len(t, repo.Wiki, 1)
	assert.Equal(t, "aGVsbG8=", repo.Wiki[0].ContentBase64)
	require.Len(t, repo.Pulls[0].Reviews, 1)
	assert.Equal(t, "main.go", repo.Pulls[0].Reviews[0].Comments[0].Path)
	require.Len(t, repo.Releases[0].Assets, 1)
	require.Len(t, repo.Packages, 1)
	assert.Equal(t, "client.bin", repo.Packages[0].Files[0].Name)
}

func TestApplyCopiesWikiAssetReviewAndGenericPackage(t *testing.T) {
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/files/notes.txt" {
			_, _ = w.Write([]byte("note"))
			return
		}
		if r.URL.Path == "/api/packages/alice/generic/client/1.0.0/client.bin" {
			_, _ = w.Write([]byte("pack"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(source.Close)
	var uploadedAsset, uploadedPackage, repositoryCreated bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			writeJSON(t, w, User{UserName: "target-admin", IsAdmin: true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/alice":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/admin/users/alice":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/repo":
			if !repositoryCreated {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, Repository{Owner: &User{UserName: "alice"}, Name: "repo", Empty: true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users/alice/repos":
			repositoryCreated = true
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Repository{Name: "repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/pulls":
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, map[string]any{"number": 2})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/pulls/2/reviews":
			assert.Equal(t, "alice", r.Header.Get("Sudo"))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/alice/repo/pulls/2":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/releases":
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Release{ID: 8, TagName: "v1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/releases/8/assets":
			assert.Equal(t, "notes.txt", r.URL.Query().Get("name"))
			uploadedAsset = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/wiki/new":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/api/packages/alice/generic/client/1.0.0/client.bin":
			uploadedPackage = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/packages/alice/generic/client/-/link/repo":
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.Method+" "+r.URL.RequestURI(), http.StatusInternalServerError)
		}
	}))
	t.Cleanup(target.Close)
	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token",
		TargetURL: target.URL, TargetToken: "target-token",
		Repositories: []string{"alice/repo"},
		Mirror:       func(context.Context, MirrorRequest) error { return nil },
	})
	require.NoError(t, err)
	result, err := migrator.Apply(context.Background(), &Plan{
		SourceURL: source.URL, SourceLogin: "root", TargetURL: target.URL,
		Users: []User{{UserName: "alice", Email: "alice@example.test", IsActive: true}},
		Repositories: []RepositoryPlan{{
			Repository: Repository{
				Owner: &User{UserName: "alice"}, Name: "repo", FullName: "alice/repo", Empty: true,
				CloneURL: source.URL + "/alice/repo.git",
			},
			Pulls: []PullRequest{{
				Number: 2, Title: "Add", State: "closed", User: &User{UserName: "alice"},
				Head: &PullRef{Ref: "topic", Repo: &Repository{FullName: "alice/repo"}},
				Base: &PullRef{Ref: "main"},
				Reviews: []PullReview{{
					Body: "ship it", State: "APPROVED", User: &User{UserName: "alice"},
					Comments: []ReviewComment{{Body: "line", Path: "main.go", LineNum: 4}},
				}},
			}},
			Releases: []Release{{
				TagName: "v1",
				Assets:  []ReleaseAsset{{Name: "notes.txt", Size: 4, DownloadURL: source.URL + "/files/notes.txt"}},
			}},
			Wiki: []WikiPage{{Title: "Home", ContentBase64: "aGVsbG8="}},
			Packages: []PackageCopy{{
				Type: "generic", Name: "client", Version: "1.0.0",
				Files: []PackageFile{{Name: "client.bin", Size: 4}},
			}},
		}},
	}, &memoryCredentials{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.WikiPagesCreated)
	assert.Equal(t, 1, result.AssetsCreated)
	assert.Equal(t, 1, result.ReviewsCreated)
	assert.Equal(t, 1, result.PackagesCopied)
	assert.True(t, uploadedAsset)
	assert.True(t, uploadedPackage)
}

func TestDownloadRejectsCrossOrigin(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(source.Close)
	client := newAPIClient(source.URL, "token", source.Client())
	_, _, err := client.downloadLimited(context.Background(), "https://evil.example/secret", 32)
	require.Error(t, err)
	temp, err := os.CreateTemp(t.TempDir(), "asset")
	require.NoError(t, err)
	require.NoError(t, temp.Close())
	err = client.uploadStream(context.Background(), http.MethodPut, "https://evil.example/upload", "application/octet-stream", temp.Name(), 0, http.StatusCreated)
	require.Error(t, err)
}
