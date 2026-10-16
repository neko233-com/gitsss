// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
		for _, match := range references.FindAllStringSubmatch(content, -1) {
			require.True(t, names[match[1]], "%s references missing template %s", path, match[1])
		}
	}
}
