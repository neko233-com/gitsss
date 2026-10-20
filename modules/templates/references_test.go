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
