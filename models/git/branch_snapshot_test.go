// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"strings"
	"testing"
	"time"

	git_model "github.com/neko233-com/gitsss/models/git"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/git"

	"github.com/stretchr/testify/require"
)

func TestBranchSnapshotCompareAndSwap(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 3})
	previous := branch.CommitID
	newer := &git.Commit{
		ID: git.MustIDFromString(strings.Repeat("2", 40)), MessageRaw: "newer",
		Committer: &git.Signature{When: time.Unix(981173106, 0)},
	}
	older := &git.Commit{
		ID: git.MustIDFromString(strings.Repeat("1", 40)), MessageRaw: "delayed",
		Committer: &git.Signature{When: newer.Committer.When},
	}
	affected, err := git_model.UpdateBranchIfUnchanged(t.Context(), branch.RepoID, 2, branch.Name, newer, previous)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	affected, err = git_model.UpdateBranchIfUnchanged(t.Context(), branch.RepoID, 1, branch.Name, older, previous)
	require.NoError(t, err)
	require.Zero(t, affected)
	actual, err := git_model.GetBranch(t.Context(), branch.RepoID, branch.Name)
	require.NoError(t, err)
	require.Equal(t, newer.ID.String(), actual.CommitID)
	require.EqualValues(t, 2, actual.PusherID)
}
