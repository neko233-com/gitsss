// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import "strings"

// RedactHTMLDocsURL hides temporary content capabilities without changing request routing.
func RedactHTMLDocsURL(raw string) string {
	const marker = "/-/docs/content/"
	before, after, found := strings.Cut(raw, marker)
	if !found {
		return raw
	}
	parts := strings.SplitN(after, "/", 3)
	if len(parts) < 2 {
		return before + marker + "[redacted]"
	}
	return before + marker + parts[0] + "/[redacted]"
}
