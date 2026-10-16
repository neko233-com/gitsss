// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package awareness

// GitLFSBytes counts measured file bytes without including other applications on the volume.
func (s Snapshot) GitLFSBytes() int64 {
	var total int64
	for _, item := range s.Usage {
		if item.Category == CategoryGitObjects || item.Category == CategoryLFSAssets {
			total = saturatingAdd(total, item.Bytes)
		}
	}
	return total
}
