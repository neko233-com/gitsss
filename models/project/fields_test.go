// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFields(t *testing.T) {
	fields, err := NormalizeFields([]Field{{
		Key: "priority", Name: "优先级", Type: "select", Items: []string{"P0", "P1"},
	}})
	require.NoError(t, err)
	encoded, err := EncodeFields(fields)
	require.NoError(t, err)
	assert.Equal(t, "priority", ParseFields(encoded)[0].Key)

	_, err = NormalizeFields([]Field{{Key: "Priority", Name: "x", Type: "text"}})
	require.Error(t, err)
	assert.Empty(t, ParseFields("{"))
}
