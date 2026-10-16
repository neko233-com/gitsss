// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package announcement

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertPublishAndDelete(t *testing.T) {
	now := time.Unix(100, 0)
	notices, err := Upsert(nil, Notice{Title: " 停机窗口 ", Body: "今晚", Published: true}, now)
	require.NoError(t, err)
	require.Len(t, notices, 1)
	assert.NotEmpty(t, notices[0].ID)
	assert.Equal(t, "停机窗口", notices[0].Title)
	assert.Equal(t, []Notice{notices[0]}, Published(notices))

	hidden := notices[0]
	hidden.Published = false
	notices, err = Upsert(notices, hidden, now)
	require.NoError(t, err)
	assert.Empty(t, Published(notices))

	notices = Delete(notices, hidden.ID)
	assert.Empty(t, notices)
	_, err = Upsert(nil, Notice{}, now)
	require.Error(t, err)
}
