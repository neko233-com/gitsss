// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/models/organization"
	user_model "github.com/neko233-com/gitsss/models/user"

	"github.com/stretchr/testify/require"
)

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
