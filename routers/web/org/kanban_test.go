// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	user_model "github.com/neko233-com/gitsss/models/user"

	"github.com/stretchr/testify/require"
)

func TestTaskMentionAssignment(t *testing.T) {
	members := user_model.UserList{{ID: 2, Name: "alice"}, {ID: 4, Name: "bob"}}
	for _, tc := range []struct {
		title, explicit string
		id              int64
	}{
		{"Task @ALICE", "", 2},
		{"Task @alice", "bob", 4},
		{"Task @alice", "-", 0},
		{"Task", "", 0},
		{"Task @alice @bob", "", 4},
	} {
		id, err := taskAssignee(tc.title, tc.explicit, members)
		require.NoError(t, err)
		require.Equal(t, tc.id, id)
	}
	_, err := taskAssignee("Task @outsider", "", members)
	require.Error(t, err)
}
