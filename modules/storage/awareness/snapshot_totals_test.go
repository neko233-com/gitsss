// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package awareness

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotGitLFSBytes(t *testing.T) {
	snapshot := Snapshot{Usage: []Usage{
		{Category: CategoryGitObjects, Bytes: 20},
		{Category: CategoryLFSAssets, Bytes: 50},
		{Category: CategoryDatabase, Bytes: 10},
		{Category: CategoryOther, Bytes: 300},
	}}
	assert.EqualValues(t, 70, snapshot.GitLFSBytes())
	snapshot.Usage = []Usage{{Category: CategoryGitObjects, Bytes: math.MaxInt64}, {Category: CategoryLFSAssets, Bytes: 1}}
	assert.EqualValues(t, math.MaxInt64, snapshot.GitLFSBytes())
}
