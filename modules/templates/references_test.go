// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bytes"
	"html/template"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/modules/translation"

	"github.com/stretchr/testify/require"
)

type repoCreateAvatarStub struct{}

func (repoCreateAvatarStub) Avatar(any, ...any) template.HTML { return "" }

func TestCreateRepositoryOwnerIdentities(t *testing.T) {
	source, err := os.ReadFile("../../templates/repo/create.tmpl")
	require.NoError(t, err)
	identity, err := os.ReadFile("../../templates/repo/create_owner_identity.tmpl")
	require.NoError(t, err)
	tmpl := template.New("repo/create").Funcs(template.FuncMap{
		"ctx": func() any {
			return struct {
				Locale      translation.MockLocale
				AvatarUtils repoCreateAvatarStub
			}{}
		},
		"svg":       func(...any) template.HTML { return "" },
		"AppSubUrl": func() string { return "" },
	})
	_, err = tmpl.Parse(string(source))
	require.NoError(t, err)
	_, err = tmpl.New("repo/create_owner_identity").Parse(string(identity))
	require.NoError(t, err)
	for _, name := range []string{"base/head", "base/footer", "base/alert", "repo/create_helper"} {
		_, err = tmpl.New(name).Parse("")
		require.NoError(t, err)
	}
	personal := &user_model.User{ID: 1, Name: "alice", FullName: "Alice <script>"}
	org := organization.OrgFromUser(&user_model.User{ID: 2, Name: "engineering", FullName: "研发团队", Type: user_model.UserTypeOrganization})
	for _, owner := range []*user_model.User{personal, org.AsUser()} {
		var output bytes.Buffer
		err = tmpl.ExecuteTemplate(&output, "repo/create", map[string]any{
			"ContextUser":         owner,
			"SignedUser":          personal,
			"Orgs":                []*organization.Organization{org},
			"CanCreateRepoInDoer": true,
			"DefaultObjectFormat": map[string]string{"Name": "sha1"},
		})
		require.NoError(t, err)
		require.Contains(t, output.String(), "repo.create.owner_personal")
		require.Contains(t, output.String(), "repo.create.owner_organization")
		require.Contains(t, output.String(), `value="`+map[int64]string{1: "1", 2: "2"}[owner.ID]+`"`)
		require.Contains(t, output.String(), ">engineering</span>")
		require.Contains(t, output.String(), "研发团队")
		require.Contains(t, output.String(), "Alice &lt;script&gt;")
		require.NotContains(t, output.String(), "<script>")
	}
}

func TestMigrationSourcesKeepNavigationContext(t *testing.T) {
	source, err := os.ReadFile("../../templates/repo/migrate/migrate.tmpl")
	require.NoError(t, err)
	icon, err := os.ReadFile("../../templates/repo/migrate/service_icon.tmpl")
	require.NoError(t, err)
	tmpl := template.New("migration").Funcs(template.FuncMap{
		"ctx":       func() any { return struct{ Locale translation.MockLocale }{} },
		"svg":       func(...any) template.HTML { return "" },
		"dict":      dict,
		"AppSubUrl": func() string { return "/git" },
	})
	_, err = tmpl.Parse(string(source))
	require.NoError(t, err)
	_, err = tmpl.New("repo/migrate/service_icon").Parse(string(icon))
	require.NoError(t, err)
	for _, name := range []string{"base/head", "base/footer", "base/alert"} {
		_, err = tmpl.New(name).Parse("")
		require.NoError(t, err)
	}
	services := append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	var output bytes.Buffer
	require.NoError(t, tmpl.ExecuteTemplate(&output, "migration", map[string]any{"Services": services, "Org": 7, "Mirror": true}))
	require.Equal(t, len(services), strings.Count(output.String(), "data-migration-source data-migration-search="))
	require.Equal(t, len(services), strings.Count(output.String(), "&org=7&mirror=true"))
	require.Contains(t, output.String(), `/git/repo/migrate?service_type=1`)
	require.Contains(t, output.String(), `migration-source-git`)
}

func TestMigrationSourceBreadcrumbSkipsLegacyOrgParameter(t *testing.T) {
	source, err := os.ReadFile("../../templates/base/head_navbar.tmpl")
	require.NoError(t, err)
	start := strings.Index(string(source), `<div class="workspace-breadcrumb">`)
	require.NotEqual(t, -1, start)
	end := strings.Index(string(source)[start:], "</div>")
	require.NotEqual(t, -1, end)
	tmpl, err := template.New("breadcrumb").Funcs(template.FuncMap{"AppSubUrl": func() string { return "" }}).Parse(string(source)[start : start+end+len("</div>")])
	require.NoError(t, err)
	_, err = tmpl.New("org/identity").Parse("{{.Name}}")
	require.NoError(t, err)
	for _, org := range []any{"7", 7} {
		var output bytes.Buffer
		require.NoError(t, tmpl.ExecuteTemplate(&output, "breadcrumb", map[string]any{"Org": org, "IsMigrationSourceChooser": true}))
		require.Contains(t, output.String(), "GitSSS")
	}
}

func TestMigrationWorkspacesPreserveIdentityAndFields(t *testing.T) {
	personal := &user_model.User{ID: 1, Name: "alice", FullName: "Alice <script>"}
	org := organization.OrgFromUser(&user_model.User{ID: 7, Name: "engineering", FullName: "研发团队", Type: user_model.UserTypeOrganization})
	services := append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	for _, service := range services {
		t.Run(service.Name(), func(t *testing.T) {
			tmpl := template.New("migration").Funcs(template.FuncMap{
				"ctx": func() any {
					return struct {
						Locale      translation.MockLocale
						AvatarUtils repoCreateAvatarStub
					}{}
				},
				"svg": func(...any) template.HTML { return "" }, "dict": dict,
				"AppSubUrl": func() string { return "/git" },
			})
			for _, name := range []string{"repo/migrate/" + service.Name(), "repo/migrate/workspace_start", "repo/migrate/destination", "repo/migrate/options", "repo/migrate/service_icon", "repo/create_owner_identity"} {
				source, err := os.ReadFile("../../templates/" + name + ".tmpl")
				require.NoError(t, err)
				_, err = tmpl.New(name).Parse(string(source))
				require.NoError(t, err)
			}
			for _, name := range []string{"base/head", "base/footer", "base/alert", "base/disable_form_autofill"} {
				_, err := tmpl.New(name).Parse("")
				require.NoError(t, err)
			}
			var output bytes.Buffer
			err := tmpl.ExecuteTemplate(&output, "repo/migrate/"+service.Name(), map[string]any{
				"service": service, "Link": "/git/repo/migrate", "SignedUser": personal,
				"ContextUser": org.AsUser(), "Orgs": []*organization.Organization{org},
				"repo_name": "retained-name", "mirror": true, "LFSActive": true, "Err_LFSEndpoint": true,
			})
			require.NoError(t, err)
			for _, field := range []string{"service", "clone_addr", "uid", "repo_name", "private", "description"} {
				require.Contains(t, output.String(), `name="`+field+`"`)
			}
			require.Contains(t, output.String(), `value="retained-name"`)
			require.Contains(t, output.String(), `org=7&mirror=true`)
			require.Contains(t, output.String(), `repo.create.owner_organization`)
			require.Contains(t, output.String(), `Alice &lt;script&gt;`)
			require.NotContains(t, output.String(), `<script>`)
		})
	}
}

func TestPublicEntranceKeepsLocalNavigationAndWorkflowControls(t *testing.T) {
	source, err := os.ReadFile("../../templates/home.tmpl")
	require.NoError(t, err)
	tmpl, err := template.New("home").Funcs(template.FuncMap{
		"ctx":            func() any { return struct{ Locale translation.MockLocale }{} },
		"svg":            func(...any) template.HTML { return "" },
		"AppSubUrl":      func() string { return "/git" },
		"AssetUrlPrefix": func() string { return "/git/assets" },
		"AppVer":         func() string { return "fixture" },
	}).Parse(string(source))
	require.NoError(t, err)
	for _, name := range []string{"base/head", "base/footer"} {
		_, err = tmpl.New(name).Parse("")
		require.NoError(t, err)
	}
	var output bytes.Buffer
	require.NoError(t, tmpl.Execute(&output, nil))
	require.Contains(t, output.String(), `href="/git/user/login"`)
	require.Contains(t, output.String(), `href="/git/help/site/actions.html"`)
	require.Equal(t, 3, strings.Count(output.String(), `aria-controls="landing-caption" disabled`))
	require.Contains(t, output.String(), `aria-live="polite"`)
	require.NotContains(t, output.String(), "<canvas")
}

func TestSignInWorkspaceKeepsExistingAuthenticationModes(t *testing.T) {
	source, err := os.ReadFile("../../templates/user/auth/signin_inner.tmpl")
	require.NoError(t, err)
	tmpl := template.New("signin").Funcs(template.FuncMap{
		"ctx": func() any { return struct{ Locale translation.MockLocale }{} },
		"svg": func(...any) template.HTML { return "" }, "AppSubUrl": func() string { return "/git" },
	})
	_, err = tmpl.Parse(string(source))
	require.NoError(t, err)
	for _, name := range []string{"base/alert", "user/auth/captcha", "user/auth/external_auth_methods"} {
		_, err = tmpl.New(name).Parse(name)
		require.NoError(t, err)
	}
	for _, binding := range []bool{false, true} {
		var output bytes.Buffer
		err = tmpl.Execute(&output, map[string]any{"EnablePasswordSignInForm": true, "LinkAccountMode": binding, "LinkAccountModeSignIn": binding, "SignInLink": "/git/user/link_account_signin", "user_name": "alice"})
		require.NoError(t, err)
		for _, field := range []string{`name="user_name"`, `name="password"`, `autocomplete="username"`, `autocomplete="current-password"`, `data-password-toggle`, `data-submit-feedback`} {
			require.Contains(t, output.String(), field)
		}
		require.Equal(t, !binding, strings.Contains(output.String(), `name="remember"`))
		require.NotContains(t, output.String(), `id="external-login-navigator"`)
	}
	var output bytes.Buffer
	require.NoError(t, tmpl.Execute(&output, map[string]any{"OAuth2Providers": []string{"configured-provider"}}))
	require.Contains(t, output.String(), "user/auth/external_auth_methods")
	require.NotContains(t, output.String(), `name="password"`)
}

func TestLanguageLegendHasOneRowPerLanguage(t *testing.T) {
	source, err := os.ReadFile("../../templates/repo/home_sidebar_bottom.tmpl")
	require.NoError(t, err)
	tmpl, err := template.New("languages").Funcs(template.FuncMap{
		"ctx":       func() any { return struct{ Locale translation.MockLocale }{} },
		"svg":       func(...any) template.HTML { return "" },
		"dict":      dict,
		"Iif":       iif,
		"DateUtils": func() any { return nil },
	}).Parse(string(source))
	require.NoError(t, err)
	_, err = tmpl.New("repo/release/label").Parse("")
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, tmpl.Execute(&output, map[string]any{"LanguageStats": []map[string]any{
		{"Language": "Go", "Percentage": 57, "Color": "#00add8"},
		{"Language": "TypeScript <tag>", "Percentage": 43, "Color": "#3178c6"},
	}}))
	require.Contains(t, output.String(), `<ul class="language-stats-details">`)
	require.Equal(t, 2, strings.Count(output.String(), `<li class="item">`))
	require.Contains(t, output.String(), `<span class="language-stats-percentage">57%</span>`)
	require.Contains(t, output.String(), "TypeScript &lt;tag&gt;")
	require.NotContains(t, output.String(), "<tag>")
}

func TestOrganizationIdentityKeepsIdentifierAndNote(t *testing.T) {
	source, err := os.ReadFile("../../templates/org/identity.tmpl")
	require.NoError(t, err)
	tmpl, err := template.New("identity").Parse(string(source))
	require.NoError(t, err)
	for _, name := range []string{"", "engineering", "研发团队", "<script>alert(1)</script>"} {
		var output bytes.Buffer
		require.NoError(t, tmpl.Execute(&output, &user_model.User{Name: "engineering", FullName: name}))
		require.Contains(t, output.String(), ">engineering</span>")
		if name == "" || name == "engineering" {
			require.NotContains(t, output.String(), "org-identity-note")
		} else {
			require.Contains(t, output.String(), "org-identity-note")
			require.Contains(t, output.String(), template.HTMLEscapeString(name))
		}
	}
}

func TestRepositoryUsesOneCompleteInterface(t *testing.T) {
	header, err := os.ReadFile("../../templates/repo/header.tmpl")
	require.NoError(t, err)
	require.NotContains(t, string(header), "gitsss-react-shell")
	require.NotContains(t, string(header), "gitsss-react.js")
	require.Contains(t, string(header), `template "org/identity"`)
}

func TestOrganizationWorkspaceEntitySelectorsExist(t *testing.T) {
	entities := map[string]reflect.Type{
		"Org":         reflect.TypeFor[*organization.Organization](),
		"ContextUser": reflect.TypeFor[*user_model.User](),
	}
	selector := regexp.MustCompile(`\.(Org|ContextUser)\.([A-Z]\w*)`)
	for _, name := range []string{"org/header", "org/menu", "org/team_filter", "base/head_navbar"} {
		content, err := os.ReadFile("../../templates/" + name + ".tmpl")
		require.NoError(t, err)
		for _, match := range selector.FindAllStringSubmatch(string(content), -1) {
			typ := entities[match[1]]
			_, method := typ.MethodByName(match[2])
			_, field := typ.Elem().FieldByName(match[2])
			require.True(t, method || field, "%s references missing %s.%s", name, match[1], match[2])
		}
	}
}

func TestBuiltinTemplateReferencesResolve(t *testing.T) {
	root := os.DirFS("../../templates")
	definitions := regexp.MustCompile(`{{-?\s*(?:define|block)\s+"([^"]+)"`)
	references := regexp.MustCompile(`{{-?\s*template\s+"([^"]+)"`)
	names := make(map[string]bool)
	sources := make(map[string]string)
	err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return err
		}
		content, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		sources[path] = string(content)
		names[strings.TrimSuffix(path, ".tmpl")] = true
		for _, match := range definitions.FindAllStringSubmatch(string(content), -1) {
			names[match[1]] = true
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, sources)
	for path, content := range sources {
		require.NotContains(t, content, "%s/docs/site/", path)
		require.NotContains(t, content, "{{AppSubUrl}}/docs/site/", path)
		for _, match := range references.FindAllStringSubmatch(content, -1) {
			require.True(t, names[match[1]], "%s references missing template %s", path, match[1])
		}
	}
}
