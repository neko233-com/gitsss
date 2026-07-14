// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/neko233-com/gitsss/server/internal/autostart"
)

func servicePlan(args []string) {
	flags := flag.NewFlagSet("service-plan", flag.ExitOnError)
	platform := flags.String("platform", runtime.GOOS, "target platform: windows, darwin or linux")
	executable := flags.String("executable", "", "absolute installed gitsss-agent path; defaults to the current executable")
	serverURL := flags.String("server", "", "gitsss API base URL")
	agentID := flags.String("id", "", "stable Agent device ID")
	tokenFile := flags.String("token-file", "", "absolute protected Agent work token path")
	workingDirectory := flags.String("working-directory", "", "absolute service working directory")
	logDirectory := flags.String("log-directory", "", "absolute log directory; used by launchd")
	flags.Parse(args)
	if strings.TrimSpace(*executable) == "" {
		resolved, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*executable = resolved
	}
	plan, err := autostart.BuildPlan(autostart.Options{
		Platform: autostart.Platform(*platform), Executable: *executable,
		ServerURL: *serverURL, AgentID: *agentID, TokenFile: *tokenFile,
		WorkingDirectory: *workingDirectory, LogDirectory: *logDirectory,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
