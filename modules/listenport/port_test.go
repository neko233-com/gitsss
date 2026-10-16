// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package listenport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePort(t *testing.T) {
	port, err := ValidatePort(" 5300 ")
	require.NoError(t, err)
	assert.Equal(t, "5300", port)
	_, err = ValidatePort("0")
	require.Error(t, err)
	_, err = ValidatePort("70000")
	require.Error(t, err)
	_, err = ValidatePort("http")
	require.Error(t, err)
}

func TestRewriteRootURLReplacesPortForTakeover(t *testing.T) {
	got, err := RewriteRootURL("http://localhost:5300/gitsss", "3000")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:3000/gitsss/", got)

	got, err = RewriteRootURL("https://git.example.com/", "5300")
	require.NoError(t, err)
	assert.Equal(t, "https://git.example.com:5300/", got)

	got, err = RewriteRootURL("http://[2001:db8::1]:5300/base", "3000")
	require.NoError(t, err)
	assert.Equal(t, "http://[2001:db8::1]:3000/base/", got)
}
