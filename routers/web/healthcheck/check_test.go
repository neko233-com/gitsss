// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheNotInitialized(t *testing.T) {
	checks := make(checks)
	assert.Equal(t, fail, checkCache(checks))
	assert.Equal(t, "cache not initialized", checks["cache:ping"][0].Output)
}
