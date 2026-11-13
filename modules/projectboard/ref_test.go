// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projectboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIssueRef(t *testing.T) {
	got, err := ParseIssueRef(" example-org/client#42 ")
	require.NoError(t, err)
	assert.Equal(t, IssueRef{Owner: "example-org", Repo: "client", Number: 42}, got)
	_, err = ParseIssueRef("client#1")
	require.Error(t, err)
	_, err = ParseIssueRef("example-org/client#0")
	require.Error(t, err)
}
