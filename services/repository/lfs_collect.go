// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/lfsgc"
	"github.com/neko233-com/gitsss/modules/setting"
)

// CollectLFSGarbage scans LFS metadata and, when Apply is set, deletes objects
// whose git pointer is gone. Storage paths and oid layout are unchanged.
func CollectLFSGarbage(ctx context.Context, raw lfsgc.Request) (lfsgc.Report, error) {
	req, err := lfsgc.Normalize(raw)
	if err != nil {
		return lfsgc.Report{}, err
	}
	if !setting.LFS.StartServer {
		return lfsgc.Report{}, errors.New("LFS support is disabled")
	}
	report := lfsgc.NewReport(req)
	opts := GarbageCollectLFSMetaObjectsOptions{
		LogDetail: func(string, ...any) {},
		AutoFix:   req.Apply,
		OlderThan: req.OlderThan,
		OnRepo: func(row LFSGCRepoResult) {
			report.Add(lfsgc.RepoResult{
				Repository: row.Repository,
				Total:      row.Total,
				Orphaned:   row.Orphaned,
				Collected:  row.Collected,
				Deleted:    row.Deleted,
				Stopped:    row.Stopped,
			})
		},
	}
	if req.Repository == "" {
		return report, GarbageCollectLFSMetaObjects(ctx, opts)
	}
	owner, name, err := lfsgc.ParseRepository(req.Repository)
	if err != nil {
		return report, err
	}
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, name)
	if err != nil {
		return report, err
	}
	return report, GarbageCollectLFSMetaObjectsForRepo(ctx, repo, opts)
}
