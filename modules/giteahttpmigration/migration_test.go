package giteahttpmigration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRejectsSameURL(t *testing.T) {
	_, err := New(Options{
		SourceURL:   "http://example.test/api/v1/",
		SourceToken: "source", TargetURL: "http://example.test", TargetToken: "target",
		Repositories: []string{"owner/repo"},
	})
	require.EqualError(t, err, "source and target URLs must differ")
}

func TestNewParsesRepositoryURL(t *testing.T) {
	migrator, err := New(Options{
		SourceURL: "http://source.test/gitea", SourceToken: "source",
		TargetURL: "http://target.test", TargetToken: "target",
		Repositories: []string{"http://source.test/gitea/engineering/repo.git", "engineering/repo"},
	})
	require.NoError(t, err)
	require.Equal(t, []RepositoryRef{{Owner: "engineering", Name: "repo"}}, migrator.repos)
}

func TestRequireSameOriginRejectsCredentialExfiltration(t *testing.T) {
	require.NoError(t, requireSameOrigin("https://git.example.test/gitea", "https://git.example.test/gitea/owner/repo.git"))
	require.Error(t, requireSameOrigin("https://git.example.test", "https://evil.example.test/owner/repo.git"))
	require.Error(t, requireSameOrigin("https://git.example.test", "https://user:secret@git.example.test/owner/repo.git"))
}

func TestDiscoverUsesReadOnlySourceRequests(t *testing.T) {
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "token source-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, []any{})
			return
		}
		switch r.URL.Path {
		case "/api/v1/user":
			writeJSON(t, w, User{UserName: "source-admin", IsAdmin: true})
		case "/api/v1/admin/users":
			writeJSON(t, w, []User{{UserName: "alice", Email: "alice@example.test", IsActive: true}})
		case "/api/v1/admin/orgs":
			writeJSON(t, w, []Organization{{Name: "engineering", Visibility: "private"}, {Name: "unrelated"}})
		case "/api/v1/repos/engineering/repo-a":
			writeJSON(t, w, Repository{
				Owner: &User{UserName: "engineering"}, Name: "repo-a", FullName: "engineering/repo-a",
				CloneURL: source.URL + "/engineering/repo-a.git", DefaultBranch: "main",
			})
		case "/api/v1/repos/engineering/repo-a/git/refs":
			writeJSON(t, w, []Reference{{Ref: "refs/heads/main", Object: &GitObject{Type: "commit", SHA: "abc123"}}})
		case "/api/v1/repos/engineering/repo-a/collaborators":
			writeJSON(t, w, []User{{UserName: "alice"}})
		case "/api/v1/repos/engineering/repo-a/collaborators/alice/permission":
			writeJSON(t, w, map[string]string{"permission": "write"})
		case "/api/v1/orgs/engineering/public_members":
			writeJSON(t, w, []User{{UserName: "alice"}})
		case "/api/v1/orgs/engineering/teams":
			writeJSON(t, w, []Team{{ID: 1, Name: "Owners", Permission: "owner"}, {ID: 2, Name: "Developers", Permission: "write"}})
		case "/api/v1/teams/1/members", "/api/v1/teams/2/members":
			writeJSON(t, w, []User{{UserName: "alice"}})
		case "/api/v1/teams/1/repos", "/api/v1/teams/2/repos":
			writeJSON(t, w, []Repository{{Owner: &User{UserName: "engineering"}, Name: "repo-a", FullName: "engineering/repo-a"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer source.Close()
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls++
		http.Error(w, "dry-run must not call target", http.StatusInternalServerError)
	}))
	defer target.Close()

	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token", TargetURL: target.URL, TargetToken: "target-token",
		Repositories: []string{"engineering/repo-a"},
	})
	require.NoError(t, err)
	plan, err := migrator.Discover(context.Background())
	require.NoError(t, err)
	assert.Zero(t, targetCalls)
	assert.Equal(t, "source-admin", plan.SourceLogin)
	require.Len(t, plan.Repositories, 1)
	assert.Equal(t, "abc123", plan.Repositories[0].Refs[0].Object.SHA)
	assert.Equal(t, []Collaborator{{UserName: "alice", Permission: "write"}}, plan.Repositories[0].Collaborators)
	require.Len(t, plan.Organizations, 1)
	assert.Equal(t, "alice", plan.Organizations[0].Owner)
	assert.Equal(t, []string{"alice"}, plan.Organizations[0].PublicMembers)
	assert.Equal(t, []RepositoryRef{{Owner: "engineering", Name: "repo-a"}}, plan.Organizations[0].Teams[0].Repositories)
}

type memoryCredentials struct {
	credentials []Credential
}

func (m *memoryCredentials) WriteCredential(credential Credential) error {
	m.credentials = append(m.credentials, credential)
	return nil
}

func TestApplyUsesPlainGitHTTPAndVerifiesRefs(t *testing.T) {
	repositoryCreated := false
	var mirrorRequest MirrorRequest
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "token target-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Query().Get("page") == "2" {
			writeJSON(t, w, []any{})
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			writeJSON(t, w, User{UserName: "target-admin", IsAdmin: true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/alice":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/engineering":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users/alice/orgs":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/engineering/teams":
			writeJSON(t, w, []Team{{ID: 10, Name: "Owners", Permission: "owner"}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/teams/10/members/alice":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/engineering/repo-a":
			if !repositoryCreated {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, Repository{Owner: &User{UserName: "engineering"}, Name: "repo-a"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/engineering/repos":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "repo-a", payload["name"])
			assert.NotContains(t, payload, "auth_password")
			repositoryCreated = true
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Repository{Owner: &User{UserName: "engineering"}, Name: "repo-a"})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/repos/engineering/repo-a":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/teams/10/repos/engineering/repo-a":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/repos/engineering/repo-a/collaborators/alice":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/admin/users/alice":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/engineering/repo-a/git/refs":
			writeJSON(t, w, []Reference{{Ref: "refs/heads/main", Object: &GitObject{Type: "commit", SHA: "abc123"}}})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.NotFoundHandler())
	defer source.Close()
	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token", TargetURL: target.URL, TargetToken: "target-token",
		Repositories: []string{"engineering/repo-a"},
		Mirror: func(_ context.Context, request MirrorRequest) error {
			mirrorRequest = request
			return nil
		},
	})
	require.NoError(t, err)
	created := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	plan := &Plan{
		SourceURL: source.URL, SourceLogin: "source-admin", TargetURL: target.URL,
		Users: []User{{UserName: "alice", Email: "alice@example.test", IsActive: true, Created: created}},
		Organizations: []OrganizationPlan{{
			Organization: Organization{Name: "engineering"}, Owner: "alice",
			Teams: []TeamPlan{{
				Team: Team{ID: 1, Name: "Owners", Permission: "owner"}, Members: []string{"alice"},
				Repositories: []RepositoryRef{{Owner: "engineering", Name: "repo-a"}},
			}},
		}},
		Repositories: []RepositoryPlan{{
			Repository:    Repository{Owner: &User{UserName: "engineering"}, Name: "repo-a", FullName: "engineering/repo-a", CloneURL: source.URL + "/engineering/repo-a.git", DefaultBranch: "main"},
			Refs:          []Reference{{Ref: "refs/heads/main", Object: &GitObject{Type: "commit", SHA: "abc123"}}},
			Collaborators: []Collaborator{{UserName: "alice", Permission: "write"}},
		}},
	}
	credentials := &memoryCredentials{}
	result, err := migrator.Apply(context.Background(), plan, credentials)
	require.NoError(t, err)
	assert.Equal(t, 1, result.UsersCreated)
	assert.Equal(t, 1, result.RepositoriesCreated)
	assert.Equal(t, 1, result.RefsVerified)
	assert.Equal(t, "source-admin", mirrorRequest.SourceLogin)
	assert.Equal(t, "source-token", mirrorRequest.SourceToken)
	assert.Equal(t, "target-admin", mirrorRequest.TargetLogin)
	assert.Equal(t, target.URL+"/engineering/repo-a.git", mirrorRequest.TargetURL)
	require.Len(t, credentials.credentials, 1)
	assert.Equal(t, "alice", credentials.credentials[0].UserName)
	assert.True(t, credentials.credentials[0].MustChangePassword)
	assert.GreaterOrEqual(t, len(credentials.credentials[0].TemporaryPassword), 40)
}

func TestJSONLCredentialFileIsExclusive(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "credentials.jsonl")
	file, err := NewJSONLCredentialFile(filePath)
	require.NoError(t, err)
	require.NoError(t, file.WriteCredential(Credential{UserName: "alice", TemporaryPassword: "secret", MustChangePassword: true}))
	require.NoError(t, file.WriteCredential(Credential{UserName: "bob", TemporaryPassword: "secret-2", MustChangePassword: true}))
	require.NoError(t, file.Close())
	_, err = NewJSONLCredentialFile(filePath)
	require.Error(t, err)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"username":"alice"`)
	assert.Equal(t, 2, strings.Count(string(content), "\n"))
	for line := range strings.SplitSeq(strings.TrimSpace(string(content)), "\n") {
		var credential Credential
		require.NoError(t, json.Unmarshal([]byte(line), &credential))
		assert.NotEmpty(t, credential.UserName)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestMirrorFailureRollsBackNewRepository(t *testing.T) {
	deleted := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/engineering/repo-a":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/engineering/repos":
			w.WriteHeader(http.StatusCreated)
			writeJSON(t, w, Repository{Name: "repo-a"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/repos/engineering/repo-a":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.NotFoundHandler())
	defer source.Close()
	migrator, err := New(Options{
		SourceURL: source.URL, SourceToken: "source-token", TargetURL: target.URL, TargetToken: "target-token",
		Repositories: []string{"engineering/repo-a"},
		Mirror: func(context.Context, MirrorRequest) error {
			return assert.AnError
		},
	})
	require.NoError(t, err)
	_, err = migrator.createRepository(context.Background(), "source-admin", "target-admin", map[string]struct{}{"engineering": {}}, RepositoryPlan{
		Repository: Repository{Owner: &User{UserName: "engineering"}, Name: "repo-a", CloneURL: source.URL + "/engineering/repo-a.git"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back")
	assert.True(t, deleted)
}

func TestAPIErrorDoesNotContainToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied "+r.Header.Get("Authorization"), http.StatusForbidden)
	}))
	defer server.Close()
	migrator, err := New(Options{
		SourceURL: server.URL, SourceToken: "top-secret-token",
		TargetURL: strings.Replace(server.URL, "127.0.0.1", "localhost", 1), TargetToken: "target",
		Repositories: []string{"owner/repo"},
	})
	require.NoError(t, err)
	_, err = migrator.Discover(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "top-secret-token")
	assert.Contains(t, err.Error(), "[REDACTED]")

	targetClient := newAPIClient(server.URL, "target-secret", server.Client(), "source-secret", "target-secret")
	err = targetClient.post(context.Background(), "/echo", map[string]string{"password": "source-secret"}, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "target-secret")
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	require.NoError(t, json.NewEncoder(w).Encode(value))
}
