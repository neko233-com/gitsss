// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	log.Printf("== TestVersion ==")
	ctx := t.Context()

	c, err := NewClient("https://example.com", SetGiteaVersion("1.12.123"))
	assert.NoError(t, err)

	assert.NoError(t, c.checkServerVersionGreaterThanOrEqual(ctx, version1_11_0))
	assert.Error(t, c.CheckServerVersionConstraint(ctx, "< 1.11.0"))

	c.serverVersion = version1_11_0
	assert.Error(t, c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0))
	c.ignoreVersion = true
	assert.NoError(t, c.checkServerVersionGreaterThanOrEqual(ctx, version1_15_0))

	c, err = NewClient("https://example.com", SetGiteaVersion("1.12.123"))
	assert.NoError(t, err)
	assert.NoError(t, c.CheckServerVersionConstraint(ctx, "=1.12.123"))
}
