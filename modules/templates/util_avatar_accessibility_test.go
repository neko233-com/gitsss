// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvatarAccessibleNameAndEscaping(t *testing.T) {
	image := string(AvatarHTML("/avatar.png", 28, "avatar", `Alice <&\"`))
	assert.Contains(t, image, `alt="Alice &lt;&amp;`)
	assert.Contains(t, image, `width="28" height="28"`)
	assert.NotContains(t, image, `<&`)
}
