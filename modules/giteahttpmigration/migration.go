// Package giteahttpmigration migrates selected Git repositories and their account/RBAC data through HTTP only.
package giteahttpmigration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
)

const pageSize = 50

type ConflictPolicy string

const (
	ConflictFail ConflictPolicy = "fail"
	ConflictSkip ConflictPolicy = "skip"
)

type Options struct {
	SourceURL       string
	SourceUser      string
	SourcePassword  string
	SourceToken     string
	TargetURL       string
	TargetUser      string
	TargetPassword  string
	TargetToken     string
	Repositories    []string
	AllRepositories bool
	ConflictPolicy  ConflictPolicy
	HTTPClient      *http.Client
	Mirror          RepositoryMirror
	MirrorCacheDir  string
	SyncExistingGit bool
}

type Migrator struct {
	source   *apiClient
	target   *apiClient
	repos    []RepositoryRef
	allRepos bool
	conflict ConflictPolicy
	mirror   RepositoryMirror
	cacheDir string
	syncGit  bool
}

type MirrorRequest struct {
	SourceURL      string
	SourceLogin    string
	SourceToken    string
	TargetURL      string
	TargetLogin    string
	TargetToken    string
	CacheDir       string
	UpdateExisting bool
}

type RepositoryMirror func(context.Context, MirrorRequest) error

type RepositoryRef struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

func (r RepositoryRef) String() string {
	return r.Owner + "/" + r.Name
}

type User struct {
	UserName      string    `json:"login"`
	FullName      string    `json:"full_name"`
	Email         string    `json:"email"`
	Visibility    string    `json:"visibility"`
	Restricted    bool      `json:"restricted"`
	IsAdmin       bool      `json:"is_admin"`
	IsActive      bool      `json:"active"`
	ProhibitLogin bool      `json:"prohibit_login"`
	Created       time.Time `json:"created"`
}

type Organization struct {
	Name                      string `json:"name"`
	FullName                  string `json:"full_name"`
	Email                     string `json:"email"`
	Description               string `json:"description"`
	Website                   string `json:"website"`
	Location                  string `json:"location"`
	Visibility                string `json:"visibility"`
	RepoAdminChangeTeamAccess bool   `json:"repo_admin_change_team_access"`
}

type Team struct {
	ID                      int64             `json:"id"`
	Name                    string            `json:"name"`
	DisplayName             string            `json:"display_name,omitempty"`
	Visibility              string            `json:"visibility,omitempty"`
	Description             string            `json:"description"`
	IncludesAllRepositories bool              `json:"includes_all_repositories"`
	Permission              string            `json:"permission"`
	Units                   []string          `json:"units,omitempty"`
	UnitsMap                map[string]string `json:"units_map,omitempty"`
	CanCreateOrgRepo        bool              `json:"can_create_org_repo"`
}

type Repository struct {
	Owner           *User  `json:"owner"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	Private         bool   `json:"private"`
	Internal        bool   `json:"internal"`
	Empty           bool   `json:"empty"`
	CloneURL        string `json:"clone_url"`
	DefaultBranch   string `json:"default_branch"`
	Archived        bool   `json:"archived"`
	HasIssues       *bool  `json:"has_issues,omitempty"`
	HasPullRequests *bool  `json:"has_pull_requests,omitempty"`
	HasReleases     *bool  `json:"has_releases,omitempty"`
	HasWiki         *bool  `json:"has_wiki,omitempty"`
	HasActions      *bool  `json:"has_actions,omitempty"`
}

type Reference struct {
	Ref    string     `json:"ref"`
	Object *GitObject `json:"object"`
}

type GitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type Collaborator struct {
	UserName   string `json:"username"`
	Permission string `json:"permission"`
}

type TeamPlan struct {
	Team         Team            `json:"team"`
	Members      []string        `json:"members"`
	Repositories []RepositoryRef `json:"repositories"`
}

type OrganizationPlan struct {
	Organization  Organization `json:"organization"`
	Owner         string       `json:"owner"`
	PublicMembers []string     `json:"public_members,omitempty"`
	Teams         []TeamPlan   `json:"teams"`
}

type RepositoryPlan struct {
	Repository         Repository     `json:"repository"`
	Refs               []Reference    `json:"refs,omitempty"`
	Collaborators      []Collaborator `json:"collaborators,omitempty"`
	OwnerCollaborators []string       `json:"owner_collaborators,omitempty"`
	Labels             []Label        `json:"labels,omitempty"`
	Milestones         []Milestone    `json:"milestones,omitempty"`
	Issues             []Issue        `json:"issues,omitempty"`
	Pulls              []PullRequest  `json:"pulls,omitempty"`
	Releases           []Release      `json:"releases,omitempty"`
	Wiki               []WikiPage     `json:"wiki,omitempty"`
	Packages           []PackageCopy  `json:"packages,omitempty"`
}

type Plan struct {
	SourceURL     string             `json:"source_url"`
	SourceLogin   string             `json:"source_login"`
	TargetURL     string             `json:"target_url"`
	Users         []User             `json:"users"`
	Organizations []OrganizationPlan `json:"organizations"`
	Repositories  []RepositoryPlan   `json:"repositories"`
	GitOnlySync   bool               `json:"git_only_sync,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type Result struct {
	UsersCreated          int      `json:"users_created"`
	UsersSkipped          int      `json:"users_skipped"`
	OrganizationsCreated  int      `json:"organizations_created"`
	OrganizationsSkipped  int      `json:"organizations_skipped"`
	TeamsCreated          int      `json:"teams_created"`
	TeamsReused           int      `json:"teams_reused"`
	RepositoriesCreated   int      `json:"repositories_created"`
	RepositoriesSkipped   int      `json:"repositories_skipped"`
	GitRepositoriesSynced int      `json:"git_repositories_synced"`
	TeamMemberships       int      `json:"team_memberships_applied"`
	TeamRepositories      int      `json:"team_repositories_applied"`
	Collaborators         int      `json:"collaborators_applied"`
	PublicMemberships     int      `json:"public_memberships_applied"`
	RefsVerified          int      `json:"refs_verified"`
	LabelsCreated         int      `json:"labels_created"`
	MilestonesCreated     int      `json:"milestones_created"`
	IssuesCreated         int      `json:"issues_created"`
	PullsCreated          int      `json:"pulls_created"`
	CommentsCreated       int      `json:"comments_created"`
	ReleasesCreated       int      `json:"releases_created"`
	AssetsCreated         int      `json:"assets_created"`
	WikiPagesCreated      int      `json:"wiki_pages_created"`
	ReviewsCreated        int      `json:"reviews_created"`
	PackagesCopied        int      `json:"packages_copied"`
	PackagesSkipped       int      `json:"packages_skipped"`
	ContentSkipped        int      `json:"content_skipped"`
	IndexRemaps           int      `json:"index_remaps"`
	Notes                 []string `json:"notes,omitempty"`
}

type Credential struct {
	UserName           string `json:"username"`
	TemporaryPassword  string `json:"temporary_password"`
	MustChangePassword bool   `json:"must_change_password"`
}

type CredentialWriter interface {
	WriteCredential(Credential) error
}

type JSONLCredentialFile struct {
	file *os.File
}

func NewJSONLCredentialFile(filePath string) (*JSONLCredentialFile, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, errors.New("credentials file path is required")
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create credentials file: %w", err)
	}
	return &JSONLCredentialFile{file: file}, nil
}

func (f *JSONLCredentialFile) WriteCredential(credential Credential) error {
	content, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	content = append(content, '\n')
	if _, err := f.file.Write(content); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("sync credentials: %w", err)
	}
	return nil
}

func (f *JSONLCredentialFile) Close() error {
	return f.file.Close()
}

func New(options Options) (*Migrator, error) {
	sourceAuth, err := resolveEndpointAuth(options.SourceUser, options.SourcePassword, options.SourceToken, "source")
	if err != nil {
		return nil, err
	}
	targetAuth, err := resolveEndpointAuth(options.TargetUser, options.TargetPassword, options.TargetToken, "target")
	if err != nil {
		return nil, err
	}
	if len(options.Repositories) == 0 && !options.AllRepositories {
		return nil, errors.New("at least one repository is required")
	}
	if options.ConflictPolicy == "" {
		options.ConflictPolicy = ConflictFail
	}
	if options.ConflictPolicy != ConflictFail && options.ConflictPolicy != ConflictSkip {
		return nil, fmt.Errorf("invalid conflict policy %q", options.ConflictPolicy)
	}

	sourceBase, err := normalizeBaseURL(options.SourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid source URL: %w", err)
	}
	targetBase, err := normalizeBaseURL(options.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	if sourceBase == targetBase {
		return nil, errors.New("source and target URLs must differ")
	}

	repositories := make([]RepositoryRef, 0, len(options.Repositories))
	seen := map[string]struct{}{}
	for _, spec := range options.Repositories {
		repo, err := parseRepository(sourceBase, spec)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(repo.String())
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		repositories = append(repositories, repo)
	}
	sort.Slice(repositories, func(i, j int) bool { return repositories[i].String() < repositories[j].String() })

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http.Transport{Proxy: nil},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	mirror := options.Mirror
	if mirror == nil {
		mirror = mirrorRepository
	}
	cacheDir := strings.TrimSpace(options.MirrorCacheDir)
	if cacheDir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("locate user cache directory: %w", err)
		}
		cacheDir = filepath.Join(base, "gitsss", "gitea-http-mirrors")
	}
	cacheDir, err = filepath.Abs(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("resolve mirror cache directory: %w", err)
	}
	secrets := []string{sourceAuth.Secret, targetAuth.Secret}
	return &Migrator{
		source:   newAuthenticatedClient(sourceBase, sourceAuth, httpClient, secrets...),
		target:   newAuthenticatedClient(targetBase, targetAuth, httpClient, secrets...),
		repos:    repositories,
		allRepos: options.AllRepositories,
		conflict: options.ConflictPolicy,
		mirror:   mirror,
		cacheDir: cacheDir,
		syncGit:  options.SyncExistingGit,
	}, nil
}

func resolveEndpointAuth(user, password, token, side string) (clientAuth, error) {
	user = strings.TrimSpace(user)
	password = strings.TrimSpace(password)
	token = strings.TrimSpace(token)
	switch {
	case password != "" && token != "":
		return clientAuth{}, fmt.Errorf("%s password and token cannot both be set", side)
	case password != "":
		if user == "" {
			return clientAuth{}, fmt.Errorf("%s username is required when a password is set", side)
		}
		return clientAuth{User: user, Secret: password, Basic: true}, nil
	case token != "":
		return clientAuth{User: user, Secret: token}, nil
	default:
		return clientAuth{}, fmt.Errorf("%s token or username and password is required", side)
	}
}

func normalizeBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("URL must contain only scheme, host, and optional base path")
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/api/v1")
	u.RawPath = ""
	return strings.TrimSuffix(u.String(), "/"), nil
}

func parseRepository(sourceBase, spec string) (RepositoryRef, error) {
	value := strings.TrimSpace(spec)
	if value == "" {
		return RepositoryRef{}, errors.New("repository must not be empty")
	}
	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err != nil {
			return RepositoryRef{}, fmt.Errorf("invalid repository URL %q: %w", spec, err)
		}
		base, err := url.Parse(sourceBase)
		if err != nil || !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) {
			return RepositoryRef{}, fmt.Errorf("repository URL %q is outside source", spec)
		}
		value = strings.TrimPrefix(u.Path, strings.TrimSuffix(base.Path, "/"))
	}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || parts[0] == "." || parts[1] == "." {
		return RepositoryRef{}, fmt.Errorf("repository %q must be owner/name", spec)
	}
	return RepositoryRef{Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")}, nil
}

func requireSameOrigin(sourceBase, cloneURL string) error {
	source, err := url.Parse(sourceBase)
	if err != nil {
		return err
	}
	clone, err := url.Parse(cloneURL)
	if err != nil {
		return err
	}
	if clone.Scheme != "http" && clone.Scheme != "https" || clone.User != nil || clone.RawQuery != "" || clone.Fragment != "" {
		return errors.New("must be an HTTP(S) URL without embedded credentials, query, or fragment")
	}
	if !strings.EqualFold(source.Scheme, clone.Scheme) || !strings.EqualFold(source.Host, clone.Host) {
		return errors.New("must use the source server origin")
	}
	sourcePath := strings.TrimSuffix(source.Path, "/")
	if sourcePath != "" && clone.Path != sourcePath && !strings.HasPrefix(clone.Path, sourcePath+"/") {
		return errors.New("must stay under the source base path")
	}
	return nil
}

func (m *Migrator) Discover(ctx context.Context) (*Plan, error) {
	plan := &Plan{SourceURL: m.source.baseURL, TargetURL: m.target.baseURL, GitOnlySync: m.syncGit}
	var sourceActor User
	if err := m.source.get(ctx, "/user", &sourceActor); err != nil {
		return nil, fmt.Errorf("read source token identity: %w", err)
	}
	if !sourceActor.IsAdmin {
		return nil, errors.New("source credentials must belong to a site administrator")
	}
	plan.SourceLogin = sourceActor.UserName
	if m.allRepos && len(m.repos) == 0 {
		refs, err := m.source.listRepositoryRefs(ctx)
		if err != nil {
			return nil, fmt.Errorf("list source repositories: %w", err)
		}
		m.repos = refs
	}
	if m.syncGit {
		for _, ref := range m.repos {
			repositoryPlan, err := m.discoverGitRepository(ctx, ref)
			if err != nil {
				return nil, err
			}
			plan.Repositories = append(plan.Repositories, repositoryPlan)
		}
		sort.Slice(plan.Repositories, func(i, j int) bool {
			return plan.Repositories[i].Repository.FullName < plan.Repositories[j].Repository.FullName
		})
		return plan, nil
	}
	if err := m.source.getAll(ctx, "/admin/users", &plan.Users); err != nil {
		return nil, fmt.Errorf("list source users (site-admin token required): %w", err)
	}
	var organizations []Organization
	if err := m.source.getAll(ctx, "/admin/orgs", &organizations); err != nil {
		return nil, fmt.Errorf("list source organizations (site-admin token required): %w", err)
	}
	userNames := make(map[string]struct{}, len(plan.Users))
	for _, user := range plan.Users {
		userNames[strings.ToLower(user.UserName)] = struct{}{}
	}
	organizationNames := make(map[string]struct{}, len(organizations))
	for _, organization := range organizations {
		organizationNames[strings.ToLower(organization.Name)] = struct{}{}
	}

	selected := map[string]struct{}{}
	for _, ref := range m.repos {
		selected[strings.ToLower(ref.String())] = struct{}{}
		repositoryPlan, err := m.discoverRepository(ctx, ref)
		if err != nil {
			return nil, err
		}
		plan.Repositories = append(plan.Repositories, repositoryPlan)
		if repositoryPlan.Repository.Internal {
			plan.Warnings = append(plan.Warnings, ref.String()+": internal visibility will migrate as private")
		}
	}

	selectedOwners := make(map[string]struct{}, len(plan.Repositories))
	for _, repository := range plan.Repositories {
		if repository.Repository.Owner == nil {
			return nil, fmt.Errorf("source repository %s has no owner", repository.Repository.FullName)
		}
		owner := strings.ToLower(repository.Repository.Owner.UserName)
		if _, isUser := userNames[owner]; !isUser {
			if _, isOrganization := organizationNames[owner]; !isOrganization {
				return nil, fmt.Errorf("source repository %s owner %s is absent from admin account listings", repository.Repository.FullName, repository.Repository.Owner.UserName)
			}
		}
		selectedOwners[owner] = struct{}{}
	}
	for _, organization := range organizations {
		if _, ok := selectedOwners[strings.ToLower(organization.Name)]; !ok {
			continue
		}
		orgPlan, err := m.discoverOrganization(ctx, organization, selected)
		if err != nil {
			return nil, err
		}
		plan.Organizations = append(plan.Organizations, orgPlan)
	}
	for _, repo := range plan.Repositories {
		for _, name := range repo.OwnerCollaborators {
			verified := false
			for _, org := range plan.Organizations {
				if !strings.EqualFold(org.Organization.Name, repo.Repository.Owner.UserName) {
					continue
				}
				for _, team := range org.Teams {
					if team.Team.Permission == "owner" {
						for _, member := range team.Members {
							verified = verified || strings.EqualFold(member, name)
						}
					}
				}
			}
			if !verified {
				return nil, fmt.Errorf("source owner collaborator %s/%s is not in the organization's owner team", repo.Repository.FullName, name)
			}
			plan.Warnings = append(plan.Warnings, repo.Repository.FullName+"/"+name+": ownership is copied through the Owners team; HTTP does not expose the masked direct grant")
		}
	}
	sort.Slice(plan.Users, func(i, j int) bool { return plan.Users[i].UserName < plan.Users[j].UserName })
	sort.Slice(plan.Organizations, func(i, j int) bool {
		return plan.Organizations[i].Organization.Name < plan.Organizations[j].Organization.Name
	})
	sort.Slice(plan.Repositories, func(i, j int) bool {
		return plan.Repositories[i].Repository.FullName < plan.Repositories[j].Repository.FullName
	})
	return plan, nil
}

func (m *Migrator) discoverGitRepository(ctx context.Context, ref RepositoryRef) (RepositoryPlan, error) {
	var repository Repository
	if err := m.source.get(ctx, repoPath(ref), &repository); err != nil {
		return RepositoryPlan{}, fmt.Errorf("read source repository %s: %w", ref, err)
	}
	if repository.Owner == nil {
		return RepositoryPlan{}, fmt.Errorf("source repository %s has no owner", ref)
	}
	if repository.CloneURL == "" {
		return RepositoryPlan{}, fmt.Errorf("source repository %s has no HTTP clone URL", ref)
	}
	if err := requireSameOrigin(m.source.baseURL, repository.CloneURL); err != nil {
		return RepositoryPlan{}, fmt.Errorf("source repository %s clone URL: %w", ref, err)
	}
	plan := RepositoryPlan{Repository: repository}
	if err := m.discoverReferences(ctx, ref, &plan); err != nil {
		return RepositoryPlan{}, err
	}
	return plan, nil
}

func (m *Migrator) discoverRepository(ctx context.Context, ref RepositoryRef) (RepositoryPlan, error) {
	var repository Repository
	if err := m.source.get(ctx, repoPath(ref), &repository); err != nil {
		return RepositoryPlan{}, fmt.Errorf("read source repository %s: %w", ref, err)
	}
	if repository.CloneURL == "" {
		return RepositoryPlan{}, fmt.Errorf("source repository %s has no HTTP clone URL", ref)
	}
	if err := requireSameOrigin(m.source.baseURL, repository.CloneURL); err != nil {
		return RepositoryPlan{}, fmt.Errorf("source repository %s clone URL: %w", ref, err)
	}
	plan := RepositoryPlan{Repository: repository}
	if err := m.discoverReferences(ctx, ref, &plan); err != nil {
		return RepositoryPlan{}, err
	}
	var collaborators []User
	if err := m.source.getAll(ctx, repoPath(ref)+"/collaborators", &collaborators); err != nil {
		return RepositoryPlan{}, fmt.Errorf("list source collaborators for %s: %w", ref, err)
	}
	for _, collaborator := range collaborators {
		if collaborator.IsAdmin {
			continue
		}
		var permission struct {
			Permission string `json:"permission"`
		}
		collaboratorPath := repoPath(ref) + "/collaborators/" + url.PathEscape(collaborator.UserName) + "/permission"
		if err := m.source.get(ctx, collaboratorPath, &permission); err != nil {
			return RepositoryPlan{}, fmt.Errorf("read collaborator permission %s/%s: %w", ref, collaborator.UserName, err)
		}
		if permission.Permission == "owner" {
			plan.OwnerCollaborators = append(plan.OwnerCollaborators, collaborator.UserName)
			continue
		}
		if permission.Permission != "read" && permission.Permission != "write" && permission.Permission != "admin" {
			return RepositoryPlan{}, fmt.Errorf("source collaborator %s/%s has invalid direct permission %q", ref, collaborator.UserName, permission.Permission)
		}
		plan.Collaborators = append(plan.Collaborators, Collaborator{UserName: collaborator.UserName, Permission: permission.Permission})
	}
	sort.Slice(plan.Collaborators, func(i, j int) bool { return plan.Collaborators[i].UserName < plan.Collaborators[j].UserName })
	if err := m.discoverRepositoryContent(ctx, ref, &plan); err != nil {
		return RepositoryPlan{}, err
	}
	return plan, nil
}

func (m *Migrator) discoverReferences(ctx context.Context, ref RepositoryRef, plan *RepositoryPlan) error {
	// Push metadata is queued, so is_empty can lag behind the actual Git refs.
	if err := m.source.get(ctx, repoPath(ref)+"/git/refs", &plan.Refs); err != nil {
		var apiErr *APIError
		if plan.Repository.Empty && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("read source refs for %s: %w", ref, err)
	}
	plan.Refs = migratedReferences(plan.Refs)
	sortReferences(plan.Refs)
	plan.Repository.Empty = len(plan.Refs) == 0
	return nil
}

func (m *Migrator) discoverOrganization(ctx context.Context, organization Organization, selected map[string]struct{}) (OrganizationPlan, error) {
	orgPlan := OrganizationPlan{Organization: organization}
	basePath := "/orgs/" + url.PathEscape(organization.Name)
	var publicMembers []User
	if err := m.source.getAll(ctx, basePath+"/public_members", &publicMembers); err != nil {
		return OrganizationPlan{}, fmt.Errorf("list public members for %s: %w", organization.Name, err)
	}
	for _, user := range publicMembers {
		orgPlan.PublicMembers = append(orgPlan.PublicMembers, user.UserName)
	}
	var teams []Team
	if err := m.source.getAll(ctx, basePath+"/teams", &teams); err != nil {
		return OrganizationPlan{}, fmt.Errorf("list teams for %s: %w", organization.Name, err)
	}
	for _, team := range teams {
		teamPlan := TeamPlan{Team: team}
		var members []User
		if err := m.source.getAll(ctx, "/teams/"+strconv.FormatInt(team.ID, 10)+"/members", &members); err != nil {
			return OrganizationPlan{}, fmt.Errorf("list members for %s/%s: %w", organization.Name, team.Name, err)
		}
		for _, member := range members {
			teamPlan.Members = append(teamPlan.Members, member.UserName)
		}
		if team.Permission == "owner" && len(teamPlan.Members) > 0 && orgPlan.Owner == "" {
			orgPlan.Owner = teamPlan.Members[0]
		}
		var repositories []Repository
		if err := m.source.getAll(ctx, "/teams/"+strconv.FormatInt(team.ID, 10)+"/repos", &repositories); err != nil {
			return OrganizationPlan{}, fmt.Errorf("list repositories for %s/%s: %w", organization.Name, team.Name, err)
		}
		for _, repository := range repositories {
			key := strings.ToLower(repository.FullName)
			if _, ok := selected[key]; ok {
				teamPlan.Repositories = append(teamPlan.Repositories, RepositoryRef{Owner: repository.Owner.UserName, Name: repository.Name})
			}
		}
		sort.Strings(teamPlan.Members)
		sort.Slice(teamPlan.Repositories, func(i, j int) bool {
			return teamPlan.Repositories[i].String() < teamPlan.Repositories[j].String()
		})
		orgPlan.Teams = append(orgPlan.Teams, teamPlan)
	}
	if orgPlan.Owner == "" {
		return OrganizationPlan{}, fmt.Errorf("organization %s has no owner-team member", organization.Name)
	}
	sort.Strings(orgPlan.PublicMembers)
	sort.Slice(orgPlan.Teams, func(i, j int) bool { return orgPlan.Teams[i].Team.Name < orgPlan.Teams[j].Team.Name })
	return orgPlan, nil
}

func (m *Migrator) Apply(ctx context.Context, plan *Plan, credentials CredentialWriter) (*Result, error) {
	if plan == nil {
		return nil, errors.New("migration plan is required")
	}
	if credentials == nil {
		return nil, errors.New("credential writer is required")
	}
	if plan.SourceURL != m.source.baseURL || plan.TargetURL != m.target.baseURL {
		return nil, errors.New("migration plan URLs do not match migrator")
	}
	if plan.SourceLogin == "" {
		return nil, errors.New("migration plan has no source token identity")
	}
	var targetActor User
	if err := m.target.get(ctx, "/user", &targetActor); err != nil {
		return nil, fmt.Errorf("read target token identity: %w", err)
	}
	if !targetActor.IsAdmin {
		return nil, errors.New("target credentials must belong to a site administrator")
	}
	result := &Result{}
	for _, user := range plan.Users {
		created, err := m.createUser(ctx, user, credentials)
		if err != nil {
			return nil, err
		}
		if created {
			result.UsersCreated++
		} else {
			result.UsersSkipped++
		}
	}

	targetTeams := map[string]map[string]Team{}
	for _, organization := range plan.Organizations {
		created, err := m.createOrganization(ctx, organization)
		if err != nil {
			return nil, err
		}
		if created {
			result.OrganizationsCreated++
		} else {
			result.OrganizationsSkipped++
		}
		teams, createdCount, reusedCount, err := m.createTeams(ctx, organization)
		if err != nil {
			return nil, err
		}
		targetTeams[strings.ToLower(organization.Organization.Name)] = teams
		result.TeamsCreated += createdCount
		result.TeamsReused += reusedCount
		for _, teamPlan := range organization.Teams {
			targetTeam, ok := findTargetTeam(teams, teamPlan.Team)
			if !ok {
				return nil, fmt.Errorf("target team not found for %s/%s", organization.Organization.Name, teamPlan.Team.Name)
			}
			for _, member := range teamPlan.Members {
				endpoint := "/teams/" + strconv.FormatInt(targetTeam.ID, 10) + "/members/" + url.PathEscape(member)
				if err := m.target.noContent(ctx, http.MethodPut, endpoint, nil); err != nil {
					return nil, fmt.Errorf("add target team member %s/%s/%s: %w", organization.Organization.Name, teamPlan.Team.Name, member, err)
				}
				result.TeamMemberships++
			}
		}
	}

	organizationNames := make(map[string]struct{}, len(plan.Organizations))
	for _, organization := range plan.Organizations {
		organizationNames[strings.ToLower(organization.Organization.Name)] = struct{}{}
	}
	createdRepos := map[string]bool{}
	for _, repository := range plan.Repositories {
		created, err := m.createRepository(ctx, plan.SourceLogin, targetActor.UserName, organizationNames, repository)
		if err != nil {
			return nil, err
		}
		if repository.Repository.Owner != nil {
			ref := RepositoryRef{Owner: repository.Repository.Owner.UserName, Name: repository.Repository.Name}
			createdRepos[strings.ToLower(ref.String())] = created
		}
		if created {
			result.RepositoriesCreated++
		} else {
			result.RepositoriesSkipped++
		}
	}

	for _, organization := range plan.Organizations {
		teams := targetTeams[strings.ToLower(organization.Organization.Name)]
		for _, teamPlan := range organization.Teams {
			targetTeam, _ := findTargetTeam(teams, teamPlan.Team)
			for _, repository := range teamPlan.Repositories {
				endpoint := "/teams/" + strconv.FormatInt(targetTeam.ID, 10) + "/repos/" + escapeRepo(repository)
				if err := m.target.noContent(ctx, http.MethodPut, endpoint, nil); err != nil {
					return nil, fmt.Errorf("add target team repository %s/%s/%s: %w", organization.Organization.Name, teamPlan.Team.Name, repository, err)
				}
				result.TeamRepositories++
			}
		}
		for _, member := range organization.PublicMembers {
			endpoint := "/orgs/" + url.PathEscape(organization.Organization.Name) + "/public_members/" + url.PathEscape(member)
			if err := m.target.noContent(ctx, http.MethodPut, endpoint, nil); err != nil {
				return nil, fmt.Errorf("publish target organization member %s/%s: %w", organization.Organization.Name, member, err)
			}
			result.PublicMemberships++
		}
	}

	for _, repository := range plan.Repositories {
		ref := RepositoryRef{Owner: repository.Repository.Owner.UserName, Name: repository.Repository.Name}
		for _, collaborator := range repository.Collaborators {
			payload := map[string]string{"permission": collaborator.Permission}
			endpoint := repoPath(ref) + "/collaborators/" + url.PathEscape(collaborator.UserName)
			if err := m.target.noContent(ctx, http.MethodPut, endpoint, payload); err != nil {
				return nil, fmt.Errorf("apply target collaborator %s/%s: %w", ref, collaborator.UserName, err)
			}
			result.Collaborators++
		}
	}

	for _, repository := range plan.Repositories {
		if repository.Repository.Owner == nil {
			return nil, errors.New("source repository has no owner")
		}
		ref := RepositoryRef{Owner: repository.Repository.Owner.UserName, Name: repository.Repository.Name}
		stats, err := m.applyRepositoryContent(ctx, ref, repository, createdRepos[strings.ToLower(ref.String())])
		if err != nil {
			return nil, err
		}
		result.LabelsCreated += stats.labels
		result.MilestonesCreated += stats.milestones
		result.IssuesCreated += stats.issues
		result.PullsCreated += stats.pulls
		result.CommentsCreated += stats.comments
		result.ReleasesCreated += stats.releases
		result.AssetsCreated += stats.assets
		result.WikiPagesCreated += stats.wikiPages
		result.ReviewsCreated += stats.reviews
		result.PackagesCopied += stats.packagesCopied
		result.PackagesSkipped += stats.packagesSkipped
		result.IndexRemaps += stats.remaps
		if stats.skipped {
			result.ContentSkipped++
		}
		result.Notes = append(result.Notes, stats.notes...)
	}

	for _, user := range plan.Users {
		if err := m.finalizeUser(ctx, user); err != nil {
			return nil, err
		}
	}
	for _, repository := range plan.Repositories {
		count, err := m.verifyRepository(ctx, repository)
		if err != nil {
			return nil, err
		}
		result.RefsVerified += count
	}
	return result, nil
}

// SyncGitRepositories updates Git refs and LFS objects on repositories that already exist on the target.
// The persistent local bare cache keeps subsequent source fetches incremental.
func (m *Migrator) SyncGitRepositories(ctx context.Context, plan *Plan) (*Result, error) {
	if !m.syncGit {
		return nil, errors.New("Git-only synchronization was not enabled")
	}
	if plan == nil {
		return nil, errors.New("migration plan is required")
	}
	if plan.SourceURL != m.source.baseURL || plan.TargetURL != m.target.baseURL || !plan.GitOnlySync {
		return nil, errors.New("Git-only synchronization plan does not match migrator")
	}
	var targetActor User
	if err := m.target.get(ctx, "/user", &targetActor); err != nil {
		return nil, fmt.Errorf("read target token identity: %w", err)
	}
	if !targetActor.IsAdmin {
		return nil, errors.New("target credentials must belong to a site-admin")
	}
	result := &Result{}
	for _, repository := range plan.Repositories {
		if repository.Repository.Owner == nil {
			return nil, errors.New("source repository has no owner")
		}
		ref := RepositoryRef{Owner: repository.Repository.Owner.UserName, Name: repository.Repository.Name}
		exists, err := m.target.exists(ctx, repoPath(ref))
		if err != nil {
			return nil, fmt.Errorf("check target repository %s: %w", ref, err)
		}
		if !exists {
			return nil, fmt.Errorf("target repository %s does not exist; run the initial import first", ref)
		}
		if !repository.Repository.Empty {
			targetURL := strings.TrimSuffix(m.target.baseURL, "/") + "/" + escapeRepo(ref) + ".git"
			if err := m.mirror(ctx, MirrorRequest{
				SourceURL: repository.Repository.CloneURL, SourceLogin: plan.SourceLogin, SourceToken: m.source.secret,
				TargetURL: targetURL, TargetLogin: targetActor.UserName, TargetToken: m.target.secret,
				CacheDir: m.cacheDir, UpdateExisting: true,
			}); err != nil {
				return nil, fmt.Errorf("sync Git repository %s: %w", ref, err)
			}
		}
		count, err := m.verifyRepository(ctx, repository)
		if err != nil {
			return nil, err
		}
		result.GitRepositoriesSynced++
		result.RefsVerified += count
	}
	return result, nil
}

func (m *Migrator) createUser(ctx context.Context, user User, credentials CredentialWriter) (bool, error) {
	exists, err := m.target.exists(ctx, "/users/"+url.PathEscape(user.UserName))
	if err != nil {
		return false, fmt.Errorf("check target user %s: %w", user.UserName, err)
	}
	if exists {
		if m.conflict == ConflictSkip {
			return false, nil
		}
		return false, fmt.Errorf("target user %s already exists", user.UserName)
	}
	password, err := randomPassword()
	if err != nil {
		return false, fmt.Errorf("generate password for %s: %w", user.UserName, err)
	}
	credential := Credential{UserName: user.UserName, TemporaryPassword: password, MustChangePassword: true}
	if err := credentials.WriteCredential(credential); err != nil {
		return false, fmt.Errorf("persist credential for %s before account creation: %w", user.UserName, err)
	}
	mustChange := true
	payload := struct {
		Username           string     `json:"username"`
		FullName           string     `json:"full_name"`
		Email              string     `json:"email"`
		Password           string     `json:"password"`
		MustChangePassword *bool      `json:"must_change_password"`
		Visibility         string     `json:"visibility"`
		Created            *time.Time `json:"created_at,omitempty"`
	}{
		Username: user.UserName, FullName: user.FullName, Email: user.Email, Password: password,
		MustChangePassword: &mustChange, Visibility: user.Visibility,
	}
	if !user.Created.IsZero() {
		payload.Created = &user.Created
	}
	if payload.Email == "" {
		payload.Email = user.UserName + "@users.invalid"
	}
	if err := m.target.post(ctx, "/admin/users", payload, nil); err != nil {
		return false, fmt.Errorf("create target user %s: %w", user.UserName, err)
	}
	return true, nil
}

func (m *Migrator) finalizeUser(ctx context.Context, user User) error {
	payload := struct {
		LoginName     string `json:"login_name"`
		Active        *bool  `json:"active"`
		Admin         *bool  `json:"admin"`
		ProhibitLogin *bool  `json:"prohibit_login"`
		Restricted    *bool  `json:"restricted"`
		Visibility    string `json:"visibility"`
	}{LoginName: user.UserName, Active: &user.IsActive, Admin: &user.IsAdmin, ProhibitLogin: &user.ProhibitLogin, Restricted: &user.Restricted, Visibility: user.Visibility}
	if err := m.target.patch(ctx, "/admin/users/"+url.PathEscape(user.UserName), payload, nil, http.StatusOK); err != nil {
		return fmt.Errorf("finalize target user %s: %w", user.UserName, err)
	}
	return nil
}

func (m *Migrator) createOrganization(ctx context.Context, plan OrganizationPlan) (bool, error) {
	name := plan.Organization.Name
	exists, err := m.target.exists(ctx, "/orgs/"+url.PathEscape(name))
	if err != nil {
		return false, fmt.Errorf("check target organization %s: %w", name, err)
	}
	if exists {
		if m.conflict == ConflictSkip {
			return false, nil
		}
		return false, fmt.Errorf("target organization %s already exists", name)
	}
	payload := map[string]any{
		"username": name, "full_name": plan.Organization.FullName, "email": plan.Organization.Email,
		"description": plan.Organization.Description, "website": plan.Organization.Website,
		"location": plan.Organization.Location, "visibility": plan.Organization.Visibility,
		"repo_admin_change_team_access": plan.Organization.RepoAdminChangeTeamAccess,
	}
	endpoint := "/admin/users/" + url.PathEscape(plan.Owner) + "/orgs"
	if err := m.target.post(ctx, endpoint, payload, nil); err != nil {
		return false, fmt.Errorf("create target organization %s with owner %s: %w", name, plan.Owner, err)
	}
	return true, nil
}

func (m *Migrator) createTeams(ctx context.Context, plan OrganizationPlan) (map[string]Team, int, int, error) {
	orgName := plan.Organization.Name
	endpoint := "/orgs/" + url.PathEscape(orgName) + "/teams"
	var existing []Team
	if err := m.target.getAll(ctx, endpoint, &existing); err != nil {
		return nil, 0, 0, fmt.Errorf("list target teams for %s: %w", orgName, err)
	}
	teams := map[string]Team{}
	for _, team := range existing {
		teams[strings.ToLower(team.Name)] = team
		if team.Permission == "owner" {
			teams["@owner"] = team
		}
	}
	created, reused := 0, 0
	for _, teamPlan := range plan.Teams {
		if _, ok := findTargetTeam(teams, teamPlan.Team); ok {
			reused++
			continue
		}
		if teamPlan.Team.Permission == "owner" {
			return nil, created, reused, fmt.Errorf("target owner team missing for %s", orgName)
		}
		payload := map[string]any{
			"name": teamPlan.Team.Name, "description": teamPlan.Team.Description,
			"display_name":              teamPlan.Team.DisplayName,
			"includes_all_repositories": teamPlan.Team.IncludesAllRepositories,
			"permission":                teamPlan.Team.Permission, "units": teamPlan.Team.Units,
			"units_map": teamPlan.Team.UnitsMap, "can_create_org_repo": teamPlan.Team.CanCreateOrgRepo,
		}
		if teamPlan.Team.Visibility != "" {
			switch teamPlan.Team.Visibility {
			case "private", "limited", "public":
				payload["visibility"] = teamPlan.Team.Visibility
			default:
				return nil, created, reused, fmt.Errorf("source team %s/%s has unknown visibility %q", orgName, teamPlan.Team.Name, teamPlan.Team.Visibility)
			}
		}
		var targetTeam Team
		if err := m.target.post(ctx, endpoint, payload, &targetTeam); err != nil {
			return nil, created, reused, fmt.Errorf("create target team %s/%s: %w", orgName, teamPlan.Team.Name, err)
		}
		teams[strings.ToLower(targetTeam.Name)] = targetTeam
		created++
	}
	return teams, created, reused, nil
}

func findTargetTeam(teams map[string]Team, source Team) (Team, bool) {
	if source.Permission == "owner" {
		team, ok := teams["@owner"]
		return team, ok
	}
	team, ok := teams[strings.ToLower(source.Name)]
	return team, ok
}

func (m *Migrator) createRepository(ctx context.Context, sourceLogin, targetLogin string, organizationNames map[string]struct{}, plan RepositoryPlan) (bool, error) {
	if plan.Repository.Owner == nil {
		return false, errors.New("source repository has no owner")
	}
	ref := RepositoryRef{Owner: plan.Repository.Owner.UserName, Name: plan.Repository.Name}
	exists, err := m.target.exists(ctx, repoPath(ref))
	if err != nil {
		return false, fmt.Errorf("check target repository %s: %w", ref, err)
	}
	if exists {
		if m.conflict == ConflictSkip {
			return false, nil
		}
		return false, fmt.Errorf("target repository %s already exists", ref)
	}
	payload := map[string]any{
		"name": ref.Name, "description": plan.Repository.Description,
		"private": plan.Repository.Private || plan.Repository.Internal, "auto_init": false,
		"default_branch": plan.Repository.DefaultBranch,
	}
	endpoint := "/admin/users/" + url.PathEscape(ref.Owner) + "/repos"
	if _, isOrganization := organizationNames[strings.ToLower(ref.Owner)]; isOrganization {
		endpoint = "/orgs/" + url.PathEscape(ref.Owner) + "/repos"
	}
	var target Repository
	if err := m.target.post(ctx, endpoint, payload, &target); err != nil {
		return false, fmt.Errorf("create target repository %s: %w", ref, err)
	}
	if !plan.Repository.Empty {
		targetCloneURL := strings.TrimSuffix(m.target.baseURL, "/") + "/" + escapeRepo(ref) + ".git"
		if err := m.mirror(ctx, MirrorRequest{
			SourceURL: plan.Repository.CloneURL, SourceLogin: sourceLogin, SourceToken: m.source.secret,
			TargetURL: targetCloneURL, TargetLogin: targetLogin, TargetToken: m.target.secret,
			CacheDir: m.cacheDir,
		}); err != nil {
			return false, m.rollbackRepository(ctx, ref, fmt.Errorf("mirror target repository %s: %w", ref, err))
		}
		if _, err := m.verifyRepository(ctx, plan); err != nil {
			return false, fmt.Errorf("wait for target push metadata %s: %w", ref, err)
		}
	}
	if plan.Repository.DefaultBranch != "" {
		if err := m.target.patch(ctx, repoPath(ref), map[string]any{"default_branch": plan.Repository.DefaultBranch}, nil, http.StatusOK); err != nil {
			return false, m.rollbackRepository(ctx, ref, fmt.Errorf("set target default branch %s: %w", ref, err))
		}
	}
	features := map[string]*bool{"has_issues": plan.Repository.HasIssues, "has_pull_requests": plan.Repository.HasPullRequests, "has_releases": plan.Repository.HasReleases, "has_wiki": plan.Repository.HasWiki, "has_actions": plan.Repository.HasActions}
	payload = map[string]any{}
	for key, enabled := range features {
		if enabled != nil {
			payload[key] = *enabled
		}
	}
	if len(payload) > 0 {
		if err := m.target.patch(ctx, repoPath(ref), payload, nil, http.StatusOK); err != nil {
			return false, m.rollbackRepository(ctx, ref, fmt.Errorf("set target features %s: %w", ref, err))
		}
	}
	if plan.Repository.Archived {
		archived := true
		if err := m.target.patch(ctx, repoPath(ref), map[string]any{"archived": &archived}, nil, http.StatusOK); err != nil {
			return false, m.rollbackRepository(ctx, ref, fmt.Errorf("archive target repository %s: %w", ref, err))
		}
	}
	return true, nil
}

func (m *Migrator) rollbackRepository(ctx context.Context, ref RepositoryRef, cause error) error {
	if err := m.target.noContent(ctx, http.MethodDelete, repoPath(ref), nil); err != nil {
		return fmt.Errorf("%w; rollback failed: %v", cause, err)
	}
	return fmt.Errorf("%w; new target repository rolled back", cause)
}

func mirrorRepository(ctx context.Context, request MirrorRequest) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git executable is required")
	}
	if _, err := exec.LookPath("git-lfs"); err != nil {
		return errors.New("git-lfs executable is required")
	}
	if strings.TrimSpace(request.CacheDir) == "" {
		return errors.New("persistent mirror cache directory is required")
	}
	cacheKey := sha256.Sum256([]byte(strings.TrimRight(request.SourceURL, "/")))
	repositoryDir := filepath.Join(request.CacheDir, hex.EncodeToString(cacheKey[:]))
	lock := mirrorLock(repositoryDir)
	lock.Lock()
	defer lock.Unlock()
	if err := os.MkdirAll(request.CacheDir, 0o700); err != nil {
		return fmt.Errorf("create mirror cache directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryDir, "HEAD")); errors.Is(err, os.ErrNotExist) {
		if err := runGit(ctx, "", "", "", "init", "--bare", "--quiet", repositoryDir); err != nil {
			return fmt.Errorf("initialize mirror cache: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect mirror cache: %w", err)
	}
	if err := runGit(ctx, "", "", "", "--git-dir", repositoryDir, "remote", "set-url", "origin", request.SourceURL); err != nil {
		if addErr := runGit(ctx, "", "", "", "--git-dir", repositoryDir, "remote", "add", "origin", request.SourceURL); addErr != nil {
			return fmt.Errorf("configure source: %w", addErr)
		}
	}
	if err := runGit(ctx, request.SourceLogin, request.SourceToken, request.SourceURL,
		"--git-dir", repositoryDir, "fetch", "--quiet", "--prune", "origin",
		"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*", "+refs/notes/*:refs/notes/*",
	); err != nil {
		return fmt.Errorf("fetch source refs: %w", err)
	}
	sourceRefs, err := gitReferenceMap(ctx, repositoryDir, "", "", "")
	if err != nil {
		return fmt.Errorf("read cached refs: %w", err)
	}
	targetKey := sha256.Sum256([]byte(request.TargetURL))
	checkpointPath := filepath.Join(repositoryDir, "last-sync-"+hex.EncodeToString(targetKey[:])+".json")
	if request.UpdateExisting {
		previous := map[string]string{}
		data, err := os.ReadFile(checkpointPath)
		if err == nil {
			if err := json.Unmarshal(data, &previous); err != nil {
				return fmt.Errorf("read mirror checkpoint: %w", err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read mirror checkpoint: %w", err)
		}
		targetRefs, err := gitReferenceMap(ctx, "", request.TargetURL, request.TargetLogin, request.TargetToken)
		if err != nil {
			return fmt.Errorf("read target refs: %w", err)
		}
		if err := validateMirrorTarget(previous, sourceRefs, targetRefs); err != nil {
			return err
		}
	}
	if err := runGit(ctx, request.SourceLogin, request.SourceToken, request.SourceURL, "-c", "lfs.url="+strings.TrimRight(request.SourceURL, "/")+"/info/lfs", "--git-dir", repositoryDir, "lfs", "fetch", "--all", "origin"); err != nil {
		return fmt.Errorf("fetch source LFS objects: %w", err)
	}
	// Publish LFS bytes before refs make their pointers visible to clients.
	if err := runGit(ctx, request.TargetLogin, request.TargetToken, request.TargetURL, "-c", "lfs.url="+strings.TrimRight(request.TargetURL, "/")+"/info/lfs", "--git-dir", repositoryDir, "lfs", "push", "--all", request.TargetURL); err != nil {
		return fmt.Errorf("push LFS objects: %w", err)
	}
	if request.UpdateExisting {
		if err := runGit(ctx, request.TargetLogin, request.TargetToken, request.TargetURL, "--git-dir", repositoryDir, "push", "--atomic", "--prune", "--quiet", request.TargetURL,
			"refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*", "refs/notes/*:refs/notes/*",
		); err != nil {
			return fmt.Errorf("push incremental Git refs: %w", err)
		}
	} else if err := runGit(ctx, request.TargetLogin, request.TargetToken, request.TargetURL, "--git-dir", repositoryDir, "push", "--mirror", "--quiet", request.TargetURL); err != nil {
		return fmt.Errorf("push Git refs: %w", err)
	}
	data, err := json.Marshal(sourceRefs)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(repositoryDir, ".last-sync-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), checkpointPath); err != nil {
		return fmt.Errorf("save mirror checkpoint: %w", err)
	}
	return nil
}

func validateMirrorTarget(previous, source, target map[string]string) error {
	if maps.Equal(target, source) || maps.Equal(target, previous) {
		return nil
	}
	return errors.New("target refs changed since the last successful sync; destination changes were preserved; reconcile them before retrying")
}

func gitReferenceMap(ctx context.Context, directory, remoteURL, login, token string) (map[string]string, error) {
	var output string
	var err error
	if remoteURL == "" {
		output, err = runGitOutput(ctx, "", "", "", "--git-dir", directory, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/", "refs/tags/", "refs/notes/")
	} else {
		output, err = runGitOutput(ctx, login, token, remoteURL, "ls-remote", "--refs", remoteURL, "refs/heads/*", "refs/tags/*", "refs/notes/*")
	}
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			refs[fields[1]] = fields[0]
		}
	}
	return refs, nil
}

var mirrorLockMap sync.Map

func mirrorLock(repositoryDir string) *sync.Mutex {
	lock, _ := mirrorLockMap.LoadOrStore(repositoryDir, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func runGit(ctx context.Context, login, token, authURL string, args ...string) error {
	_, err := executeGit(ctx, login, token, authURL, false, args...)
	return err
}

func runGitOutput(ctx context.Context, login, token, authURL string, args ...string) (string, error) {
	return executeGit(ctx, login, token, authURL, true, args...)
}

func executeGit(ctx context.Context, login, token, authURL string, capture bool, args ...string) (string, error) {
	extraHeader := ""
	credential := ""
	if login != "" || token != "" {
		credential = base64.StdEncoding.EncodeToString([]byte(login + ":" + token))
		extraHeader = "Authorization: Basic " + credential
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = sanitizedGitEnvironment(extraHeader, authURL)
	output := &limitedBuffer{limit: 64 * 1024}
	if capture {
		output.limit = 8 * 1024 * 1024
	}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		message := output.String()
		for _, secret := range []string{token, credential, extraHeader} {
			if secret != "" {
				message = strings.ReplaceAll(message, secret, "[REDACTED]")
			}
		}
		return "", fmt.Errorf("git %s failed: %w: %s", args[0], err, strings.TrimSpace(message))
	}
	if capture && output.truncated {
		return "", errors.New("Git reference output exceeded 8 MiB; refs were not published")
	}
	return output.String(), nil
}

func sanitizedGitEnvironment(extraHeader, authURL string) []string {
	environment := make([]string, 0, len(os.Environ())+7)
	blocked := []string{"GIT_CONFIG_", "GIT_TERMINAL_PROMPT=", "GIT_ASKPASS=", "GIT_LFS_SKIP_SMUDGE=", "GIT_ALLOW_PROTOCOL=", "GIT_TRACE"}
	for _, item := range os.Environ() {
		upperItem := strings.ToUpper(item)
		discard := false
		for _, prefix := range blocked {
			if strings.HasPrefix(upperItem, prefix) {
				discard = true
				break
			}
		}
		if !discard {
			environment = append(environment, item)
		}
	}
	configCount := "2"
	config := []string{
		"GIT_CONFIG_KEY_0=http.followRedirects",
		"GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=credential.helper",
		"GIT_CONFIG_VALUE_1=",
	}
	if extraHeader != "" {
		configCount = "3"
		config = append(config, "GIT_CONFIG_KEY_2=http."+authURL+".extraHeader", "GIT_CONFIG_VALUE_2="+extraHeader)
	}
	environment = append(environment, "GIT_CONFIG_COUNT="+configCount)
	environment = append(environment, config...)
	return append(environment,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_ALLOW_PROTOCOL=http:https",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"GIT_LFS_SKIP_SMUDGE=1",
	)
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(content []byte) (int, error) {
	originalLength := len(content)
	remaining := b.limit - b.buffer.Len()
	if len(content) > remaining {
		b.truncated = true
	}
	if remaining > 0 {
		if len(content) > remaining {
			content = content[:remaining]
		}
		_, _ = b.buffer.Write(content)
	}
	return originalLength, nil
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

func (m *Migrator) verifyRepository(ctx context.Context, plan RepositoryPlan) (int, error) {
	ref := RepositoryRef{Owner: plan.Repository.Owner.UserName, Name: plan.Repository.Name}
	deadline := time.Now().Add(5 * time.Minute)
	for {
		var lastReason string
		var target Repository
		if err := m.target.get(ctx, repoPath(ref), &target); err != nil {
			return 0, fmt.Errorf("verify target repository %s: %w", ref, err)
		}
		if target.Empty == plan.Repository.Empty {
			if plan.Repository.Empty {
				return 0, nil
			}
			var refs []Reference
			err := m.target.get(ctx, repoPath(ref)+"/git/refs", &refs)
			if err == nil {
				refs = migratedReferences(refs)
				sortReferences(refs)
				if equalReferences(plan.Refs, refs) {
					return len(refs), nil
				}
				lastReason = "source and target refs differ"
			} else {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
					return 0, fmt.Errorf("verify target refs for %s: %w", ref, err)
				}
				lastReason = "target refs not synchronized"
			}
		} else {
			lastReason = "empty state differs"
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("verify target repository %s: %s after 5m", ref, lastReason)
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, ctx.Err()
		case <-timer.C:
		}
	}
}

func randomPassword() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sortReferences(refs []Reference) {
	sort.Slice(refs, func(i, j int) bool { return refs[i].Ref < refs[j].Ref })
}

func migratedReferences(refs []Reference) []Reference {
	result := make([]Reference, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref.Ref, "refs/heads/") || strings.HasPrefix(ref.Ref, "refs/tags/") || strings.HasPrefix(ref.Ref, "refs/notes/") {
			result = append(result, ref)
		}
	}
	return result
}

func equalReferences(left, right []Reference) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Ref != right[i].Ref || left[i].Object == nil || right[i].Object == nil ||
			left[i].Object.SHA != right[i].Object.SHA || left[i].Object.Type != right[i].Object.Type {
			return false
		}
	}
	return true
}

func repoPath(ref RepositoryRef) string {
	return "/repos/" + escapeRepo(ref)
}

func escapeRepo(ref RepositoryRef) string {
	return url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Name)
}

type clientAuth struct {
	User   string
	Secret string
	Basic  bool
}

type apiClient struct {
	baseURL string
	apiURL  string
	user    string
	secret  string
	basic   bool
	sudo    string
	secrets []string
	http    *http.Client
}

func newAPIClient(baseURL, secret string, httpClient *http.Client, secrets ...string) *apiClient {
	return newAuthenticatedClient(baseURL, clientAuth{Secret: secret}, httpClient, secrets...)
}

func newAuthenticatedClient(baseURL string, auth clientAuth, httpClient *http.Client, secrets ...string) *apiClient {
	return &apiClient{
		baseURL: baseURL, apiURL: strings.TrimSuffix(baseURL, "/") + "/api/v1",
		user: auth.User, secret: auth.Secret, basic: auth.Basic, secrets: secrets, http: httpClient,
	}
}

func (c *apiClient) asUser(user string) *apiClient {
	clone := *c
	clone.sudo = user
	return &clone
}

func (c *apiClient) get(ctx context.Context, endpoint string, output any) error {
	return c.do(ctx, http.MethodGet, endpoint, nil, output, http.StatusOK)
}

func (c *apiClient) post(ctx context.Context, endpoint string, input, output any) error {
	return c.do(ctx, http.MethodPost, endpoint, input, output, http.StatusCreated)
}

func (c *apiClient) patch(ctx context.Context, endpoint string, input, output any, status int) error {
	return c.do(ctx, http.MethodPatch, endpoint, input, output, status)
}

func (c *apiClient) noContent(ctx context.Context, method, endpoint string, input any) error {
	return c.do(ctx, method, endpoint, input, nil, http.StatusNoContent)
}

func (c *apiClient) exists(ctx context.Context, endpoint string) (bool, error) {
	err := c.do(ctx, http.MethodGet, endpoint, nil, nil, http.StatusOK)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

func (c *apiClient) listRepositoryRefs(ctx context.Context) ([]RepositoryRef, error) {
	seen := map[string]struct{}{}
	var refs []RepositoryRef
	for pageNumber := 1; pageNumber <= 2000; pageNumber++ {
		var page struct {
			Data []struct {
				Name  string `json:"name"`
				Owner struct {
					UserName string `json:"login"`
				} `json:"owner"`
			} `json:"data"`
		}
		endpoint := "/repos/search?private=true&page=" + strconv.Itoa(pageNumber) + "&limit=" + strconv.Itoa(pageSize)
		if err := c.get(ctx, endpoint, &page); err != nil {
			return nil, err
		}
		if len(page.Data) == 0 {
			break
		}
		for _, repository := range page.Data {
			ref := RepositoryRef{Owner: repository.Owner.UserName, Name: repository.Name}
			key := strings.ToLower(ref.String())
			if ref.Owner == "" || ref.Name == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			refs = append(refs, ref)
		}
		if len(page.Data) < pageSize {
			break
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	return refs, nil
}

func (c *apiClient) getAll(ctx context.Context, endpoint string, output any) error {
	value := reflectSlice(output)
	for pageNumber := 1; ; pageNumber++ {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		pageEndpoint := endpoint + separator + "page=" + strconv.Itoa(pageNumber) + "&limit=" + strconv.Itoa(pageSize)
		pageValue := value.newPage()
		if err := c.get(ctx, pageEndpoint, pageValue.pointer()); err != nil {
			return err
		}
		count := pageValue.length()
		value.append(pageValue)
		if count == 0 {
			return nil
		}
	}
}

type sliceValue struct {
	value any
}

func reflectSlice(output any) sliceValue {
	return sliceValue{value: output}
}

func (s sliceValue) newPage() sliceValue {
	switch target := s.value.(type) {
	case *[]User:
		page := make([]User, 0)
		return sliceValue{value: &page}
	case *[]Organization:
		page := make([]Organization, 0)
		return sliceValue{value: &page}
	case *[]Team:
		page := make([]Team, 0)
		return sliceValue{value: &page}
	case *[]Repository:
		page := make([]Repository, 0)
		return sliceValue{value: &page}
	case *[]Label:
		page := make([]Label, 0)
		return sliceValue{value: &page}
	case *[]Milestone:
		page := make([]Milestone, 0)
		return sliceValue{value: &page}
	case *[]Issue:
		page := make([]Issue, 0)
		return sliceValue{value: &page}
	case *[]PullRequest:
		page := make([]PullRequest, 0)
		return sliceValue{value: &page}
	case *[]IssueComment:
		page := make([]IssueComment, 0)
		return sliceValue{value: &page}
	case *[]Release:
		page := make([]Release, 0)
		return sliceValue{value: &page}
	case *[]WikiPage:
		page := make([]WikiPage, 0)
		return sliceValue{value: &page}
	case *[]PullReview:
		page := make([]PullReview, 0)
		return sliceValue{value: &page}
	case *[]PackageCopy:
		page := make([]PackageCopy, 0)
		return sliceValue{value: &page}
	case *[]string:
		page := make([]string, 0)
		return sliceValue{value: &page}
	default:
		panic(fmt.Sprintf("unsupported paginated output %T", target))
	}
}

func (s sliceValue) pointer() any { return s.value }

func (s sliceValue) length() int {
	switch value := s.value.(type) {
	case *[]User:
		return len(*value)
	case *[]Organization:
		return len(*value)
	case *[]Team:
		return len(*value)
	case *[]Repository:
		return len(*value)
	case *[]Label:
		return len(*value)
	case *[]Milestone:
		return len(*value)
	case *[]Issue:
		return len(*value)
	case *[]PullRequest:
		return len(*value)
	case *[]IssueComment:
		return len(*value)
	case *[]Release:
		return len(*value)
	case *[]WikiPage:
		return len(*value)
	case *[]PullReview:
		return len(*value)
	case *[]PackageCopy:
		return len(*value)
	case *[]string:
		return len(*value)
	default:
		panic(fmt.Sprintf("unsupported paginated output %T", value))
	}
}

func (s sliceValue) append(page sliceValue) {
	switch target := s.value.(type) {
	case *[]User:
		*target = append(*target, *page.value.(*[]User)...)
	case *[]Organization:
		*target = append(*target, *page.value.(*[]Organization)...)
	case *[]Team:
		*target = append(*target, *page.value.(*[]Team)...)
	case *[]Repository:
		*target = append(*target, *page.value.(*[]Repository)...)
	case *[]Label:
		*target = append(*target, *page.value.(*[]Label)...)
	case *[]Milestone:
		*target = append(*target, *page.value.(*[]Milestone)...)
	case *[]Issue:
		*target = append(*target, *page.value.(*[]Issue)...)
	case *[]PullRequest:
		*target = append(*target, *page.value.(*[]PullRequest)...)
	case *[]IssueComment:
		*target = append(*target, *page.value.(*[]IssueComment)...)
	case *[]Release:
		*target = append(*target, *page.value.(*[]Release)...)
	case *[]WikiPage:
		*target = append(*target, *page.value.(*[]WikiPage)...)
	case *[]PullReview:
		*target = append(*target, *page.value.(*[]PullReview)...)
	case *[]PackageCopy:
		*target = append(*target, *page.value.(*[]PackageCopy)...)
	case *[]string:
		*target = append(*target, *page.value.(*[]string)...)
	default:
		panic(fmt.Sprintf("unsupported paginated output %T", target))
	}
}

type APIError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s returned %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

func (c *apiClient) do(ctx context.Context, method, endpoint string, input, output any, expectedStatus int) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	requestURL := c.apiURL + "/" + strings.TrimPrefix(path.Clean("/"+endpoint), "/")
	if strings.Contains(endpoint, "?") {
		cleanPath, query, _ := strings.Cut(endpoint, "?")
		requestURL = c.apiURL + "/" + strings.TrimPrefix(path.Clean("/"+cleanPath), "/") + "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.basic {
		req.SetBasicAuth(c.user, c.secret)
	} else {
		req.Header.Set("Authorization", "token "+c.secret)
	}
	if c.sudo != "" {
		req.Header.Set("Sudo", c.sudo)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("perform %s %s: %w", method, requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		bodyText := strings.TrimSpace(string(message))
		for _, secret := range c.secrets {
			if secret != "" {
				bodyText = strings.ReplaceAll(bodyText, secret, "[REDACTED]")
			}
		}
		return &APIError{Method: method, URL: requestURL, StatusCode: resp.StatusCode, Body: bodyText}
	}
	if output == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(output); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, requestURL, err)
	}
	return nil
}
