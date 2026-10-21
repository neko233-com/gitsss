// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayScriptKeepsCustomHookDispatch(t *testing.T) {
	names, templates, _ := getHookTemplates()
	for index, name := range names[:3] {
		script := hookRelayScript(templates[index], name == "update")
		assert.Contains(t, script, `for hook in "${GIT_DIR}/hooks/${hookname}.d/"*; do`)
		assert.Contains(t, script, `unset GITSSS_HOOK_TOKEN`)
		assert.Contains(t, script, `if [ "${result}" -eq 75 ]; then`)
		assert.Contains(t, script, `[ -n "${GITSSS_HOOK_TOKEN}" ]`)
		assert.Contains(t, script, `exitcodes="${exitcodes} ${result}"`)
		assert.Contains(t, script, `for i in ${exitcodes}; do`)
		assert.NotContains(t, script, "$(dirname $0)")
		if name == "update" {
			assert.Contains(t, script, `"${hookname}" "$1" "$2" "$3"`)
		} else {
			assert.Contains(t, script, `echo "${data}" |`)
		}
	}
}

func TestRelayFileConcurrentCreationAndTamper(t *testing.T) {
	directory := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() { assert.NoError(t, ensureRelayFile(directory, "pre-receive", script)) })
	}
	workers.Wait()
	path := filepath.Join(directory, "pre-receive")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, script, string(data))
	require.NoError(t, os.WriteFile(path, []byte("independent change\n"), 0o700))
	assert.Error(t, ensureRelayFile(directory, "pre-receive", script))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "independent change\n", string(data))
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, strings.HasPrefix(entry.Name(), ".hook-"))
	}
}

func TestManagedHookMustMatchRegularExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed")
	data := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o700))
	assert.True(t, regularHookMatches(path, data))
	assert.False(t, regularHookMatches(path, data+"changed"))
	assert.False(t, regularHookMatches(path+"missing", data))
}

func TestRelayLinkConcurrentCreationAndTamper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps the shell proxy")
	}
	directory := t.TempDir()
	helper := filepath.Join(directory, "gitsss-hook")
	require.NoError(t, os.WriteFile(helper, []byte("fixture"), 0o700))
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() { assert.NoError(t, ensureRelayLink(directory, "pre-receive", helper)) })
	}
	workers.Wait()
	target, err := os.Readlink(filepath.Join(directory, "pre-receive"))
	require.NoError(t, err)
	assert.Equal(t, helper, target)
	assert.Error(t, ensureRelayLink(directory, "pre-receive", helper+"-changed"))
}
