// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webtheme

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseThemeMetaInfo(t *testing.T) {
	m := parseThemeMetaInfoToMap(`gitea-theme-meta-info {
	--k1: "v1";
	--k2: "v\"2";
	--k3: 'v3';
	--k4: 'v\'4';
	--k5: v5;
}`)
	assert.Equal(t, map[string]string{
		"--k1": "v1",
		"--k2": `v"2`,
		"--k3": "v3",
		"--k4": "v'4",
		"--k5": "v5",
	}, m)

	// if an auto theme imports others, the meta info should be extracted from the last one
	// the meta in imported themes should be ignored to avoid incorrect overriding
	m = parseThemeMetaInfoToMap(`
@media (prefers-color-scheme: dark) { gitea-theme-meta-info { --k1: foo; } }
@media (prefers-color-scheme: light) { gitea-theme-meta-info { --k1: bar; } }
gitea-theme-meta-info {
	--k2: real;
}`)
	assert.Equal(t, map[string]string{"--k2": "real"}, m)

	// compressed CSS, no trailing semicolon
	m = parseThemeMetaInfoToMap(`gitea-theme-meta-info{--k1:"v1"}`)
	assert.Equal(t, map[string]string{"--k1": "v1"}, m)
	m = parseThemeMetaInfoToMap(`gitea-theme-meta-info{--k1:"v1";--k2:"v2"}`)
	assert.Equal(t, map[string]string{"--k1": "v1", "--k2": "v2"}, m)
}

func TestLoadGitSSSThemeFromAssets(t *testing.T) {
	staticRootPath, err := filepath.Abs("../..")
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.StaticRootPath, staticRootPath)()

	themeList, themeMap := loadThemesFromAssets(false)

	assert.NotEmpty(t, themeList)
	theme, ok := themeMap["gitsss-light"]
	require.True(t, ok)
	assert.Equal(t, "gitsss-light", theme.InternalName)
}
