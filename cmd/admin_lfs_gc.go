// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"time"

	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/lfsgc"
	"github.com/neko233-com/gitsss/modules/storage"
	repo_service "github.com/neko233-com/gitsss/services/repository"

	"github.com/urfave/cli/v3"
)

func newLFSGCCommand() *cli.Command {
	return &cli.Command{
		Name:  "lfs-gc",
		Usage: "Scan or delete unreferenced Git LFS objects",
		Description: `Dry-run is the default and prints a JSON report.
--apply deletes LFS objects whose git pointer is gone and whose metadata is older than --older-than (default 168h).
Objects newer than 24h are refused unless --force is set, because an upload can land before its pointer.
Git object paths and LFS oid layout are not rewritten.`,
		Action: runAdminLFSGC,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "apply", Usage: "Delete unreferenced LFS objects. Omit to scan only"},
			&cli.DurationFlag{Name: "older-than", Value: lfsgc.DefaultAge, Usage: "Only consider metadata older than this duration"},
			&cli.StringFlag{Name: "repo", Usage: "Limit the scan to owner/name"},
			&cli.BoolFlag{Name: "force", Usage: "Allow --older-than below 24h"},
		},
	}
}

func runAdminLFSGC(ctx context.Context, c *cli.Command) error {
	if err := initDB(ctx); err != nil {
		return err
	}
	if err := git.InitSimple(); err != nil {
		return err
	}
	if err := storage.Init(); err != nil {
		return err
	}
	olderThan := time.Time{}
	if c.IsSet("older-than") {
		olderThan = time.Now().Add(-c.Duration("older-than"))
	}
	report, err := repo_service.CollectLFSGarbage(ctx, lfsgc.Request{
		Apply:      c.Bool("apply"),
		OlderThan:  olderThan,
		Repository: c.String("repo"),
		Force:      c.Bool("force"),
	})
	if err != nil && report.OlderThan == "" {
		if writeErr := writeIndentedJSON(c.Writer, map[string]string{"error": err.Error()}); writeErr != nil {
			return writeErr
		}
		return err
	}
	if writeErr := writeIndentedJSON(c.Writer, report); writeErr != nil {
		return writeErr
	}
	return err
}
