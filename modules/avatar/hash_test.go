// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar_test

import (
	"testing"

	"github.com/neko233-com/gitsss/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func Test_HashAvatar(t *testing.T) {
	// Fixed bytes keep this contract independent of PNG encoder versions.
	data := []byte("avatar fixture")

	assert.Equal(t, "609e3b48b4cc2e2ce879c52d6266a45a7e3143200cddbda6a2a7c1ea3e07cded", avatar.HashAvatar(1, data))
	assert.Equal(t, "3b287e9a97adc3a8e203835c8cd85fe2441cdc24736caf0666f8f8ee0f818fc8", avatar.HashAvatar(8, data))
	assert.Equal(t, "2f5d4bda535f541d715de48c7591e6c83052a75258b1961df3bab5e530115eee", avatar.HashAvatar(1024, data))
	assert.Equal(t, "161178642c7d59eb25a61dddced5e6b66eae1c70880d5f148b1b497b767e72d9", avatar.HashAvatar(1024, []byte{}))
}
