// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListRepoActionsRunsOptionsQueryEncode(t *testing.T) {
	// Test empty options
	opt := ListRepoActionsRunsOptions{}
	opt.setDefaults()
	query := opt.QueryEncode()
	assert.Contains(t, query, "page=1")

	// Test with branch filter
	opt = ListRepoActionsRunsOptions{
		Branch: "main",
	}
	opt.setDefaults()
	query = opt.QueryEncode()
	assert.Contains(t, query, "branch=main")

	// Test with event filter
	opt = ListRepoActionsRunsOptions{
		Event: "push",
	}
	opt.setDefaults()
	query = opt.QueryEncode()
	assert.Contains(t, query, "event=push")

	// Test with status filter
	opt = ListRepoActionsRunsOptions{
		Status: "success",
	}
	opt.setDefaults()
	query = opt.QueryEncode()
	assert.Contains(t, query, "status=success")

	// Test with all filters
	opt = ListRepoActionsRunsOptions{
		Branch: "develop",
		Event:  "pull_request",
		Status: "failure",
	}
	opt.Page = 2
	opt.PageSize = 50
	query = opt.QueryEncode()
	assert.Contains(t, query, "branch=develop")
	assert.Contains(t, query, "event=pull_request")
	assert.Contains(t, query, "status=failure")
	assert.Contains(t, query, "page=2")
	assert.Contains(t, query, "limit=50")
}
