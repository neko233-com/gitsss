// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"strings"
	"testing"
	"time"

	git_model "github.com/neko233-com/gitsss/models/git"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/git"

	"github.com/stretchr/testify/require"
)

func TestBranchSnapshotCurrentRef(t *testing.T) {
	for _, scenario := range []string{"superseded", "deleted", "force-push", "current"} {
		t.Run(scenario, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			branch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{ID: 3})
			next := strings.Repeat("3", 40)
			loads := 0
			getCommit := func(oid string) (*git.Commit, error) {
				loads++
				return &git.Commit{
					ID: git.MustIDFromString(oid), MessageRaw: "new state",
					Committer: &git.Signature{When: time.Unix(1, 0)},
				}, nil
			}
			getCurrent := func(name string) (string, error) {
				require.Equal(t, branch.Name, name)
				switch scenario {
				case "superseded":
					return strings.Repeat("4", 40), nil
				case "deleted":
					return "", git.ErrNotExist{ID: name}
				default:
					return next, nil
				}
			}
			require.NoError(t, SyncBranchesToDBAtCurrentRef(t.Context(), branch.RepoID, 2,
				[]string{branch.Name}, []string{next}, getCommit, getCurrent))
			actual, err := git_model.GetBranch(t.Context(), branch.RepoID, branch.Name)
			require.NoError(t, err)
			if scenario == "superseded" || scenario == "deleted" {
				require.Equal(t, branch.CommitID, actual.CommitID)
				require.Zero(t, loads)
			} else {
				require.Equal(t, next, actual.CommitID)
				require.Equal(t, 1, loads)
			}
		})
	}
}
