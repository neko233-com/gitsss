// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/neko233-com/gitsss/server/internal/autostart"
)

func servicePlan(args []string) {
	plan, err := parseServicePlan("service-plan", args, "", "", nil)
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

func serviceInstall(args []string) {
	tokenEnvironment := "GITSSS_AGENT_WORK_TOKEN"
	plan, err := parseServicePlan("service-install", args, "", "", func(flags *flag.FlagSet) {
		flags.StringVar(&tokenEnvironment, "token-env", tokenEnvironment, "environment variable containing the Agent work token")
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	token, err := resolveAgentWorkToken("", tokenEnvironment, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := autostart.Install(ctx, plan, token); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("installed gitsss Agent service for %s\n", plan.Platform)
}

func serviceUninstall(args []string) {
	plan, err := parseServicePlan("service-uninstall", args, "http://localhost", "uninstall", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := autostart.Uninstall(ctx, plan); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("uninstalled gitsss Agent service for %s\n", plan.Platform)
}

func parseServicePlan(name string, args []string, defaultServer, defaultAgentID string, extend func(*flag.FlagSet)) (autostart.Plan, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	platform := flags.String("platform", runtime.GOOS, "target platform: windows, darwin or linux")
	executable := flags.String("executable", "", "absolute installed gitsss-agent path; defaults to the current executable")
	serverURL := flags.String("server", defaultServer, "gitsss API base URL")
	agentID := flags.String("id", defaultAgentID, "stable Agent device ID")
	tokenFile := flags.String("token-file", "", "absolute protected Agent work token path")
	workingDirectory := flags.String("working-directory", "", "absolute service working directory")
	logDirectory := flags.String("log-directory", "", "absolute log directory; used by launchd")
	if extend != nil {
		extend(flags)
	}
	if err := flags.Parse(args); err != nil {
		return autostart.Plan{}, err
	}
	if strings.TrimSpace(*executable) == "" {
		resolved, err := os.Executable()
		if err != nil {
			return autostart.Plan{}, err
		}
		*executable = resolved
	}
	return autostart.BuildPlan(autostart.Options{
		Platform: autostart.Platform(*platform), Executable: *executable,
		ServerURL: *serverURL, AgentID: *agentID, TokenFile: *tokenFile,
		WorkingDirectory: *workingDirectory, LogDirectory: *logDirectory,
	})
}
