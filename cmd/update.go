// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/updatechecker"

	"github.com/urfave/cli/v3"
)

func newUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Check GitHub releases for gitsss and download a checksum-verified package",
		Description: `Probes api.github.com and the configured HTTP mirrors in parallel.
The fastest healthy mirror is preferred for downloads. When GitHub itself answers, its manifest and sha256 are authoritative and a tampered mirror is rejected.
An empty mirror list uses the built-in mainland-China prefix proxy. Pass --mirror repeatedly to replace that list.

The checker does not replace the running server. download writes a verified file and stops.`,
		Commands: []*cli.Command{
			newUpdateCheckCommand(),
			newUpdateDownloadCommand(),
		},
	}
}

func updateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Write one JSON document to stdout"},
		&cli.BoolFlag{Name: "no-github", Usage: "Do not contact GitHub; use only an intranet manifest"},
		&cli.StringFlag{Name: "repo", Usage: "GitHub repository owner/name"},
		&cli.StringFlag{Name: "endpoint", Usage: "Intranet manifest URL for this run"},
		&cli.StringSliceFlag{Name: "mirror", Usage: "HTTP mirror prefix or {url} template. Repeat to set the full list"},
		&cli.DurationFlag{Name: "probe-timeout", Usage: "Per-endpoint probe timeout"},
		&cli.BoolFlag{Name: "save", Usage: "Write the mirror list, repository, and CHECK_GITHUB to the config file"},
		&cli.BoolFlag{Name: "enable-schedule", Usage: "With --save, enable cron.update_checker on the next start"},
	}
}

func newUpdateCheckCommand() *cli.Command {
	return &cli.Command{
		Name:   "check",
		Usage:  "Show the latest gitsss release and which mirror answered",
		Flags:  updateFlags(),
		Action: runUpdateCheck,
	}
}

func newUpdateDownloadCommand() *cli.Command {
	flags := updateFlags()
	flags = append(flags,
		&cli.StringFlag{Name: "asset", Usage: "Release asset file name. Defaults to the package for this OS and architecture"},
		&cli.StringFlag{Name: "output", Usage: "Destination file, or a directory that will contain the asset", Required: true},
	)
	return &cli.Command{
		Name:   "download",
		Usage:  "Download one release asset and reject it when the sha256 does not match",
		Flags:  flags,
		Action: runUpdateDownload,
	}
}

func runUpdateCheck(ctx context.Context, c *cli.Command) error {
	opts := updateOptions(c)
	if err := saveUpdateOptions(c, opts); err != nil {
		return writeUpdateError(c, err)
	}
	report, err := updatechecker.Check(ctx, opts)
	if c.Bool("json") {
		if writeErr := writeIndentedJSON(c.Writer, reportOrError(report, err)); writeErr != nil {
			return writeErr
		}
		return err
	}
	printUpdateReport(c.Writer, report)
	return err
}

func runUpdateDownload(ctx context.Context, c *cli.Command) error {
	opts := updateOptions(c)
	if err := saveUpdateOptions(c, opts); err != nil {
		return writeUpdateError(c, err)
	}
	result, err := updatechecker.Download(ctx, opts, c.String("asset"), c.String("output"))
	if c.Bool("json") {
		if writeErr := writeIndentedJSON(c.Writer, result); writeErr != nil {
			return writeErr
		}
		return err
	}
	printUpdateReport(c.Writer, result.Report)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.Writer, "verified: %s\nsha256:   %s\nsource:   %s\nbytes:    %d\n", result.Output, result.SHA256, result.Source, result.Bytes)
	return nil
}

func updateOptions(c *cli.Command) updatechecker.Options {
	opts := updatechecker.OptionsFromConfig()
	opts.CurrentVersion = setting.AppVer
	opts.CheckGitHub = !c.Bool("no-github")
	if c.IsSet("repo") {
		opts.GitHubRepo = c.String("repo")
	}
	if c.IsSet("endpoint") {
		opts.HTTPEndpoint = c.String("endpoint")
	}
	if mirrors := c.StringSlice("mirror"); len(mirrors) > 0 {
		opts.Mirrors = mirrors
	}
	if c.IsSet("probe-timeout") {
		opts.ProbeTimeout = c.Duration("probe-timeout")
	}
	return opts
}

func saveUpdateOptions(c *cli.Command, opts updatechecker.Options) error {
	if !c.Bool("save") {
		return nil
	}
	mirrors := opts.Mirrors
	if len(mirrors) == 0 && opts.CheckGitHub {
		mirrors = []string{updatechecker.DefaultMirror}
	}
	settings := updatechecker.Settings{
		Mirrors:     mirrors,
		GitHubRepo:  opts.GitHubRepo,
		CheckGitHub: opts.CheckGitHub,
	}
	if c.Bool("enable-schedule") {
		enabled := true
		settings.Enabled = &enabled
	}
	return updatechecker.SaveSettings(settings)
}

func reportOrError(report updatechecker.Report, err error) any {
	if err != nil && report.CheckedAt == "" {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	if err != nil && report.Error == "" {
		report.Error = err.Error()
		report.OK = false
	}
	return report
}

func writeUpdateError(c *cli.Command, err error) error {
	if c.Bool("json") {
		if writeErr := writeIndentedJSON(c.Writer, map[string]any{"ok": false, "error": err.Error()}); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func printUpdateReport(writer io.Writer, report updatechecker.Report) {
	if report.CheckedAt == "" && report.Error == "" && report.LatestVersion == "" {
		return
	}
	_, _ = fmt.Fprintf(writer, "current:  %s\nlatest:   %s\nupdate:   %s\n", report.CurrentVersion, report.LatestVersion, updateWord(report.NeedUpdate))
	trust := "mirror manifest"
	if report.ManifestTrusted {
		trust = "trusted"
	}
	_, _ = fmt.Fprintf(writer, "manifest: %s (%s)\n", report.ManifestSource, trust)
	if report.SelectedMirror != "" {
		_, _ = fmt.Fprintf(writer, "mirror:   %s\n", report.SelectedMirror)
	}
	if len(report.DownloadOrder) > 0 {
		_, _ = fmt.Fprintf(writer, "download: %s\n", strings.Join(report.DownloadOrder, ", "))
	}
	if report.HTMLURL != "" {
		_, _ = fmt.Fprintf(writer, "url:      %s\n", report.HTMLURL)
	}
	if report.Note != "" {
		_, _ = fmt.Fprintf(writer, "note:     %s\n", report.Note)
	}
	if report.Error != "" {
		_, _ = fmt.Fprintf(writer, "error:    %s\n", report.Error)
	}
	if len(report.Probes) > 0 {
		_, _ = fmt.Fprintln(writer, "probes:")
		for _, probe := range report.Probes {
			state := "FAIL"
			if probe.OK {
				state = "OK  "
			}
			selected := ""
			if probe.Selected {
				selected = " selected"
			}
			_, _ = fmt.Fprintf(writer, "  %s %5dms  %-16s %s%s\n", state, probe.LatencyMS, probe.Name, probe.Error, selected)
		}
	}
}

func updateWord(need bool) string {
	if need {
		return "available"
	}
	return "not required"
}
