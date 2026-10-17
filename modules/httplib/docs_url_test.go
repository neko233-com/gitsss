// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactHTMLDocsURL(t *testing.T) {
	assert.Equal(t, "/owner/repo/-/docs/content/abc/[redacted]", RedactHTMLDocsURL("/owner/repo/-/docs/content/abc/fixture-secret/index.html?q=secret"))
	assert.Equal(t, "/owner/repo?tab=files", RedactHTMLDocsURL("/owner/repo?tab=files"))
}
