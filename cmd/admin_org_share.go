// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	share_model "github.com/neko233-com/gitsss/models/orgshare"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/orgshare"
	"github.com/neko233-com/gitsss/modules/setting"
	share_service "github.com/neko233-com/gitsss/services/orgshare"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
)

func newOrgShareCommand() *cli.Command {
	orgFlag := &cli.StringFlag{Name: "org", Usage: "Organization name", Required: true}
	return &cli.Command{
		Name:  "org-share",
		Usage: "Manage organization file shares stored outside Git and LFS",
		Commands: []*cli.Command{
			{Name: "list", Usage: "List one folder as JSON", Action: runOrgShareList, Flags: []cli.Flag{orgFlag, &cli.Int64Flag{Name: "parent"}}},
			{Name: "text", Usage: "Store a text note", Action: runOrgShareText, Flags: []cli.Flag{
				orgFlag, &cli.StringFlag{Name: "name", Required: true}, &cli.StringFlag{Name: "body", Required: true},
				&cli.Int64Flag{Name: "parent"}, &cli.Int64Flag{Name: "uploader", Required: true}, &cli.BoolFlag{Name: "permanent"},
			}},
			{Name: "put", Usage: "Upload a file with parallel chunks", Action: runOrgSharePut, Flags: []cli.Flag{
				orgFlag, &cli.StringFlag{Name: "file", Required: true}, &cli.StringFlag{Name: "name"},
				&cli.Int64Flag{Name: "parent"}, &cli.Int64Flag{Name: "uploader", Required: true}, &cli.BoolFlag{Name: "permanent"},
				&cli.BoolFlag{Name: "resume"}, &cli.IntFlag{Name: "parallel", Value: orgshare.DefaultParallel},
				&cli.Int64Flag{Name: "chunk-bytes", Value: orgshare.DefaultChunkBytes},
			}},
			{Name: "get", Usage: "Download a file with parallel ranges", Action: runOrgShareGet, Flags: []cli.Flag{
				orgFlag, &cli.Int64Flag{Name: "id", Required: true}, &cli.StringFlag{Name: "output", Required: true},
				&cli.IntFlag{Name: "parallel", Value: orgshare.DefaultParallel}, &cli.Int64Flag{Name: "chunk-bytes", Value: orgshare.DefaultChunkBytes},
			}},
			{Name: "delete", Usage: "Delete one object", Action: runOrgShareDelete, Flags: []cli.Flag{orgFlag, &cli.Int64Flag{Name: "id", Required: true}}},
			{Name: "clear", Usage: "Delete every object in the organization share", Action: runOrgShareClear, Flags: []cli.Flag{orgFlag}},
			{Name: "cleanup", Usage: "Delete expired objects", Action: runOrgShareCleanup},
			{Name: "keep", Usage: "Mark one object permanent or restore the 30 day lifetime", Action: runOrgShareKeep, Flags: []cli.Flag{
				orgFlag, &cli.Int64Flag{Name: "id", Required: true}, &cli.BoolFlag{Name: "permanent"},
			}},
			{Name: "quota", Usage: "Set one organization's cap in bytes. Zero restores the 100 GB default", Action: runOrgShareQuota, Flags: []cli.Flag{
				orgFlag, &cli.Int64Flag{Name: "bytes", Required: true},
			}},
			{Name: "usage", Usage: "Show disk space and every organization", Action: runOrgShareUsage},
			{Name: "push", Usage: "Upload through the HTTP API with parallel chunks", Action: runOrgSharePush, Flags: []cli.Flag{
				&cli.StringFlag{Name: "url", Required: true}, &cli.StringFlag{Name: "token-file", Required: true},
				&cli.StringFlag{Name: "file", Required: true}, &cli.StringFlag{Name: "name"}, &cli.BoolFlag{Name: "permanent"},
				&cli.IntFlag{Name: "parallel", Value: orgshare.DefaultParallel}, &cli.Int64Flag{Name: "chunk-bytes", Value: orgshare.DefaultChunkBytes},
			}},
			{Name: "pull", Usage: "Download through the HTTP API with parallel Range requests", Action: runOrgSharePull, Flags: []cli.Flag{
				&cli.StringFlag{Name: "url", Required: true}, &cli.StringFlag{Name: "token-file", Required: true},
				&cli.StringFlag{Name: "output", Required: true}, &cli.IntFlag{Name: "parallel", Value: orgshare.DefaultParallel},
				&cli.Int64Flag{Name: "chunk-bytes", Value: orgshare.DefaultChunkBytes},
			}},
		},
	}
}

func runOrgShareList(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	items, err := share_service.List(ctx, orgID, c.Int64("parent"))
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, items)
}

func runOrgShareText(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	item, err := share_service.CreateText(ctx, orgID, c.Int64("uploader"), c.Int64("parent"), c.String("name"), c.String("body"), c.Bool("permanent"))
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, item)
}

func runOrgSharePut(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	file, err := os.Open(c.String("file"))
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	name := c.String("name")
	if name == "" {
		name = info.Name()
	}
	upload, err := share_service.BeginUpload(ctx, orgID, c.Int64("uploader"), c.Int64("parent"), name, info.Size(), c.Bool("permanent"), c.Bool("resume"))
	if err != nil {
		return err
	}
	if err := copyShareChunks(ctx, file, info.Size(), orgshare.NormalizeChunk(c.Int64("chunk-bytes")), orgshare.NormalizeParallel(c.Int("parallel")), func(chunkCtx context.Context, start, length int64, reader io.Reader) error {
		_, err := share_service.Append(chunkCtx, orgID, upload.ID, start, reader, length)
		return err
	}); err != nil {
		return err
	}
	item, err := share_service.Complete(ctx, orgID, upload.ID)
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, item)
}

func runOrgShareGet(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	item, source, err := share_service.Open(ctx, orgID, c.Int64("id"))
	if err != nil {
		return err
	}
	if item.Kind == share_model.KindText {
		if source != nil {
			_ = source.Close()
		}
		if err := os.WriteFile(c.String("output"), []byte(item.Text), 0o600); err != nil {
			return err
		}
		return writeIndentedJSON(c.Writer, map[string]any{"id": item.ID, "name": item.Name, "output": c.String("output")})
	}
	if source == nil {
		return errors.New("folders are not downloaded")
	}
	defer source.Close()
	output, err := os.OpenFile(c.String("output"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	if err := output.Truncate(item.Size); err != nil {
		return err
	}
	if err := copyShareChunks(ctx, source, item.Size, orgshare.NormalizeChunk(c.Int64("chunk-bytes")), orgshare.NormalizeParallel(c.Int("parallel")), func(_ context.Context, start, length int64, reader io.Reader) error {
		_, err := io.Copy(io.NewOffsetWriter(output, start), reader)
		return err
	}); err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, map[string]any{"id": item.ID, "name": item.Name, "size": item.Size, "output": c.String("output")})
}

func runOrgShareDelete(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	if err := share_service.Delete(ctx, orgID, c.Int64("id")); err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, map[string]any{"deleted": c.Int64("id")})
}

func runOrgShareClear(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	if err := share_service.Clear(ctx, orgID); err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, map[string]any{"cleared": c.String("org")})
}

func runOrgShareCleanup(ctx context.Context, c *cli.Command) error {
	if err := prepareShareDB(ctx); err != nil {
		return err
	}
	if err := share_service.Cleanup(ctx, time.Now()); err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, map[string]any{"cleaned": true})
}

func runOrgShareKeep(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	item, err := share_service.SetPermanent(ctx, orgID, c.Int64("id"), c.Bool("permanent"))
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, item)
}

func runOrgShareQuota(ctx context.Context, c *cli.Command) error {
	orgID, err := prepareOrgShare(ctx, c.String("org"))
	if err != nil {
		return err
	}
	if err := share_service.SetQuota(ctx, orgID, c.Int64("bytes")); err != nil {
		return err
	}
	usage, err := share_service.FillUsage(ctx, orgID, c.String("org"))
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, usage)
}

func runOrgShareUsage(ctx context.Context, c *cli.Command) error {
	if err := prepareShareDB(ctx); err != nil {
		return err
	}
	rows := make([]struct {
		ID   int64
		Name string
	}, 0)
	if err := db.GetEngine(ctx).Table("user").Where("type = ?", user_model.UserTypeOrganization).Cols("id", "name").Asc("lower_name").Find(&rows); err != nil {
		return err
	}
	usages := make([]share_service.Usage, 0, len(rows))
	for _, row := range rows {
		usage, err := share_service.FillUsage(ctx, row.ID, row.Name)
		if err != nil {
			return err
		}
		usages = append(usages, usage)
	}
	report, err := share_service.Report(ctx, usages)
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, report)
}

func runOrgSharePush(ctx context.Context, c *cli.Command) error {
	token, err := readSecretFile(c.String("token-file"))
	if err != nil {
		return err
	}
	item, err := (orgshare.HTTPClient{
		BaseURL: c.String("url"), Token: strings.TrimSpace(token),
		Parallel: c.Int("parallel"), ChunkSize: c.Int64("chunk-bytes"),
	}).PutFile(ctx, c.String("file"), c.String("name"), c.Bool("permanent"))
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, item)
}

func runOrgSharePull(ctx context.Context, c *cli.Command) error {
	token, err := readSecretFile(c.String("token-file"))
	if err != nil {
		return err
	}
	if err := (orgshare.HTTPClient{
		Token: strings.TrimSpace(token), Parallel: c.Int("parallel"), ChunkSize: c.Int64("chunk-bytes"),
	}).GetFile(ctx, c.String("url"), c.String("output")); err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, map[string]any{"output": c.String("output")})
}

func copyShareChunks(ctx context.Context, file io.ReaderAt, size, chunk int64, parallel int, write func(ctx context.Context, start, length int64, reader io.Reader) error) error {
	spans := orgshare.PlanChunks(size, chunk)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallel)
	for _, span := range spans {
		group.Go(func() error {
			return write(groupCtx, span.Start, span.End-span.Start, io.NewSectionReader(file, span.Start, span.End-span.Start))
		})
	}
	return group.Wait()
}

func prepareOrgShare(ctx context.Context, name string) (int64, error) {
	if err := prepareShareDB(ctx); err != nil {
		return 0, err
	}
	org, err := organization.GetOrgByName(ctx, name)
	if err != nil {
		return 0, err
	}
	return org.ID, nil
}

func prepareShareDB(ctx context.Context) error {
	setting.LoadSettings()
	return initDB(ctx)
}
