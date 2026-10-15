// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/gitsss/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEndpointAuth(t *testing.T) {
	_, err := New(Options{
		SourceURL: "http://source.test", SourcePassword: "pw", SourceToken: "token",
		TargetURL: "http://target.test", TargetToken: "target",
		Repositories: []string{"owner/repo"},
	})
	require.EqualError(t, err, "source password and token cannot both be set")

	_, err = New(Options{
		SourceURL: "http://source.test", SourcePassword: "pw",
		TargetURL: "http://target.test", TargetToken: "target",
		Repositories: []string{"owner/repo"},
	})
	require.EqualError(t, err, "source username is required when a password is set")
}

func TestPasswordAuthUsesBasicHeader(t *testing.T) {
	var got string
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		http.Error(w, "denied source-password", http.StatusForbidden)
	}))
	t.Cleanup(source.Close)
	target := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(target.Close)

	migrator, err := New(Options{
		SourceURL: source.URL, SourceUser: "root", SourcePassword: "source-password",
		TargetURL: target.URL, TargetUser: "root", TargetPassword: "target-password",
		Repositories: []string{"owner/repo"},
	})
	require.NoError(t, err)
	_, err = migrator.Discover(context.Background())
	require.Error(t, err)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("root:source-password")), got)
	assert.NotContains(t, err.Error(), "source-password")
	assert.NotContains(t, err.Error(), "target-password")
	assert.Contains(t, err.Error(), "[REDACTED]")
}

func TestDiscoverReadsIssuesAndPulls(t *testing.T) {
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
		case "/api/v1/repos/alice/repo/collaborators":
			writeJSON(t, w, []User{})
		case "/api/v1/repos/alice/repo/labels":
			writeJSON(t, w, []Label{{Name: "bug", Color: "#ee0000"}})
		case "/api/v1/repos/alice/repo/milestones":
			writeJSON(t, w, []Milestone{{Title: "v1", State: "open"}})
		case "/api/v1/repos/alice/repo/issues":
			require.Equal(t, "issues", r.URL.Query().Get("type"))
			writeJSON(t, w, []Issue{{
				Number: 4, Title: "Crash", State: "open", User: &User{UserName: "alice"},
				Labels: []Label{{Name: "bug"}}, Milestone: &Milestone{Title: "v1"},
			}})
		case "/api/v1/repos/alice/repo/issues/4/comments":
			writeJSON(t, w, []IssueComment{{Body: "hello", User: &User{UserName: "alice"}}})
		case "/api/v1/repos/alice/repo/pulls":
			writeJSON(t, w, []PullRequest{{
				Number: 5, Title: "Add shader", State: "closed", Merged: true,
				Head: &PullRef{Ref: "feature", Repo: &Repository{FullName: "alice/repo"}},
				Base: &PullRef{Ref: "main"},
			}})
		case "/api/v1/repos/alice/repo/issues/5/comments":
			writeJSON(t, w, []IssueComment{})
		case "/api/v1/repos/alice/repo/releases":
			writeJSON(t, w, []Release{{TagName: "v1.0.0", Name: "First", Body: "notes", Target: "main"}})
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
	require.Len(t, plan.Repositories, 1)
	repo := plan.Repositories[0]
	require.Len(t, repo.Issues, 1)
	assert.Equal(t, "hello", repo.Issues[0].Comments[0].Body)
	assert.Equal(t, "bug", repo.Labels[0].Name)
	require.Len(t, repo.Pulls, 1)
	assert.True(t, repo.Pulls[0].Merged)
	assert.Equal(t, "feature", repo.Pulls[0].Head.Ref)
	require.Len(t, repo.Releases, 1)
	assert.Equal(t, "v1.0.0", repo.Releases[0].TagName)
}

func TestApplyCopiesIssuesAndPulls(t *testing.T) {
	repositoryCreated := false
	var sudoHeader, pullHeadName string
	var issueClosed bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "token target-token", r.Header.Get("Authorization"))
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
			writeJSON(t, w, Repository{Owner: &User{UserName: "alice"}, Name: "repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/labels":
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Label{ID: 7, Name: "bug"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/milestones":
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Milestone{ID: 3, Title: "v1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/issues":
			sudoHeader = r.Header.Get("Sudo")
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			issueClosed, _ = payload["closed"].(bool)
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, map[string]any{"number": 4})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/issues/4/comments":
			assert.Equal(t, "alice", r.Header.Get("Sudo"))
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/pulls":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			pullHeadName, _ = payload["head"].(string)
			assert.Equal(t, "main", payload["base"])
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, map[string]any{"number": 5})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/alice/repo/pulls/5":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/releases":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "v1.0.0", payload["tag_name"])
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(source.Close)

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
			Labels:     []Label{{Name: "bug", Color: "#ee0000"}},
			Milestones: []Milestone{{Title: "v1", State: "open"}},
			Issues: []Issue{{
				Number: 4, Title: "Crash", State: "closed", User: &User{UserName: "alice"},
				Labels: []Label{{Name: "bug"}}, Milestone: &Milestone{Title: "v1"},
				Comments: []IssueComment{{Body: "hello", User: &User{UserName: "alice"}}},
			}},
			Pulls: []PullRequest{{
				Number: 5, Title: "Add shader", State: "closed", Merged: true, User: &User{UserName: "alice"},
				Head: &PullRef{Ref: "feature", Repo: &Repository{FullName: "bob/repo", Owner: &User{UserName: "bob"}}},
				Base: &PullRef{Ref: "main"},
			}},
			Releases: []Release{{TagName: "v1.0.0", Name: "First", Target: "main"}},
		}},
	}, &memoryCredentials{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.IssuesCreated)
	assert.Equal(t, 1, result.PullsCreated)
	assert.Equal(t, 1, result.CommentsCreated)
	assert.Equal(t, 1, result.LabelsCreated)
	assert.Equal(t, 1, result.MilestonesCreated)
	assert.Equal(t, 1, result.ReleasesCreated)
	assert.Equal(t, "alice", sudoHeader)
	assert.True(t, issueClosed)
	assert.Equal(t, "bob:feature", pullHeadName)
	assert.Zero(t, result.IndexRemaps)
}

func TestExistingRepositorySkipsTrackerContent(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			writeJSON(t, w, User{UserName: "target-admin", IsAdmin: true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/alice":
			writeJSON(t, w, User{UserName: "alice"})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/admin/users/alice":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/alice/repo":
			writeJSON(t, w, Repository{Owner: &User{UserName: "alice"}, Name: "repo", Empty: true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/alice/repo/issues":
			t.Fatal("existing repository must not receive copied issues")
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(source.Close)
	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token",
		TargetURL: target.URL, TargetToken: "target-token",
		Repositories:   []string{"alice/repo"},
		ConflictPolicy: ConflictSkip,
	})
	require.NoError(t, err)
	result, err := migrator.Apply(context.Background(), &Plan{
		SourceURL: source.URL, SourceLogin: "root", TargetURL: target.URL,
		Users: []User{{UserName: "alice", IsActive: true}},
		Repositories: []RepositoryPlan{{
			Repository: Repository{Owner: &User{UserName: "alice"}, Name: "repo", FullName: "alice/repo", Empty: true},
			Issues:     []Issue{{Number: 1, Title: "Keep", State: "open"}},
		}},
	}, &memoryCredentials{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.ContentSkipped)
	assert.Zero(t, result.IssuesCreated)
	require.NotEmpty(t, result.Notes)
}

func TestDuplicateTrackerNumber(t *testing.T) {
	migrator, err := New(Options{
		SourceURL: "http://source.test", SourceToken: "source",
		TargetURL: "http://target.test", TargetToken: "target",
		Repositories: []string{"owner/repo"},
	})
	require.NoError(t, err)
	_, err = migrator.applyRepositoryContent(context.Background(), RepositoryRef{Owner: "owner", Name: "repo"}, RepositoryPlan{
		Issues: []Issue{{Number: 1, Title: "a"}, {Number: 1, Title: "b"}},
	}, true)
	require.ErrorContains(t, err, "two tracker items numbered 1")
}

func TestPullHeadAndLabelColor(t *testing.T) {
	head, err := pullHead(RepositoryRef{Owner: "alice", Name: "repo"}, PullRequest{
		Number: 2,
		Head:   &PullRef{Ref: "topic", Repo: &Repository{FullName: "alice/repo"}},
		Base:   &PullRef{Ref: "main"},
	})
	require.NoError(t, err)
	assert.Equal(t, "topic", head)
	assert.Equal(t, "ee0000", normalizeLabelColor("#ee0000"))
	assert.Equal(t, "ededed", normalizeLabelColor("nope"))
}
