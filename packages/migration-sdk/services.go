package gitea

//go:generate go run ./tools/gen-client-service-wrappers

// ActionsService handles Gitea Actions endpoints.
type ActionsService struct{ *Client }

// ActivityService handles activity feed endpoints.
type ActivityService struct{ *Client }

// ActivityPubService handles ActivityPub federation endpoints.
type ActivityPubService struct{ *Client }

// AdminService handles admin endpoints.
type AdminService struct{ *Client }

// GitService handles low-level git data endpoints.
type GitService struct{ *Client }

// HooksService handles hook endpoints.
type HooksService struct{ *Client }

// IssuesService handles issue endpoints.
type IssuesService struct{ *Client }

// RenderService handles markup rendering endpoints.
type RenderService struct{ *Client }

// MetaService handles instance metadata endpoints.
type MetaService struct{ *Client }

// NotificationsService handles notification endpoints.
type NotificationsService struct{ *Client }

// OAuth2Service handles OAuth2 application endpoints.
type OAuth2Service struct{ *Client }

// OrganizationsService handles organization endpoints.
type OrganizationsService struct{ *Client }

// PackagesService handles package registry endpoints.
type PackagesService struct{ *Client }

// PullRequestsService handles pull request endpoints.
type PullRequestsService struct{ *Client }

// RepositoriesService handles repository endpoints.
type RepositoriesService struct{ *Client }

// ReleasesService handles repository release endpoints.
type ReleasesService struct{ *Client }

// WikiService handles repository wiki endpoints.
type WikiService struct{ *Client }

// SettingsService handles global settings endpoints.
type SettingsService struct{ *Client }

// TemplatesService handles instance template endpoints.
type TemplatesService struct{ *Client }

// UsersService handles user endpoints.
type UsersService struct{ *Client }

func (c *Client) initServices() {
	c.Actions = &ActionsService{Client: c}
	c.Activity = &ActivityService{Client: c}
	c.ActivityPub = &ActivityPubService{Client: c}
	c.Admin = &AdminService{Client: c}
	c.Git = &GitService{Client: c}
	c.Hooks = &HooksService{Client: c}
	c.Issues = &IssuesService{Client: c}
	c.Render = &RenderService{Client: c}
	c.Meta = &MetaService{Client: c}
	c.Notifications = &NotificationsService{Client: c}
	c.OAuth2 = &OAuth2Service{Client: c}
	c.Organizations = &OrganizationsService{Client: c}
	c.Packages = &PackagesService{Client: c}
	c.PullRequests = &PullRequestsService{Client: c}
	c.Repositories = &RepositoriesService{Client: c}
	c.Releases = &ReleasesService{Client: c}
	c.Settings = &SettingsService{Client: c}
	c.Templates = &TemplatesService{Client: c}
	c.Users = &UsersService{Client: c}
	c.Wiki = &WikiService{Client: c}
}
