// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgrep

import (
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestIndexSettingToGitGrepPathspecList(t *testing.T) {
	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("a"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("b"))()
	assert.Equal(t, []string{":(glob)a", ":(glob,exclude)b"}, indexSettingToGitGrepPathspecList())
}
