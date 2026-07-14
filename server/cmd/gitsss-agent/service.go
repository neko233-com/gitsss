// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	launcherExecutable := flags.String("launcher-executable", "", "absolute stable gitsss-launcher path; auto-detected beside an installed Agent")
	stateDirectory := flags.String("state-directory", "", "absolute immutable launcher state directory")
	version := flags.String("version", "", "installed package version; auto-detected from BUILDINFO.json")
	commit := flags.String("commit", "", "installed package commit; auto-detected from BUILDINFO.json")
	architecture := flags.String("arch", "", "target architecture: amd64 or arm64")
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
	if strings.TrimSpace(*launcherExecutable) == "" && *platform == runtime.GOOS {
		candidate := filepath.Join(filepath.Dir(*executable), launcherFilename(runtime.GOOS))
		if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() {
			*launcherExecutable = candidate
		}
	}
	if strings.TrimSpace(*launcherExecutable) != "" {
		if strings.TrimSpace(*version) == "" || strings.TrimSpace(*commit) == "" || strings.TrimSpace(*architecture) == "" {
			build, err := readServiceBuildInfo(filepath.Join(filepath.Dir(*executable), "BUILDINFO.json"))
			if err == nil && build.GOOS == *platform {
				if *version == "" {
					*version = build.Version
				}
				if *commit == "" {
					*commit = build.Commit
				}
				if *architecture == "" {
					*architecture = build.GOARCH
				}
			}
		}
		if *architecture == "" && *platform == runtime.GOOS {
			*architecture = runtime.GOARCH
		}
	}
	return autostart.BuildPlan(autostart.Options{
		Platform: autostart.Platform(*platform), Executable: *executable,
		ServerURL: *serverURL, AgentID: *agentID, TokenFile: *tokenFile,
		WorkingDirectory: *workingDirectory, LogDirectory: *logDirectory,
		LauncherExecutable: *launcherExecutable, StateDirectory: *stateDirectory,
		Version: *version, Commit: *commit, Architecture: *architecture,
	})
}

type serviceBuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	GOOS    string `json:"goos"`
	GOARCH  string `json:"goarch"`
}

func readServiceBuildInfo(filePath string) (serviceBuildInfo, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return serviceBuildInfo{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 16<<10 {
		return serviceBuildInfo{}, fmt.Errorf("BUILDINFO.json must be a small regular file")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return serviceBuildInfo{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var build serviceBuildInfo
	if err := decoder.Decode(&build); err != nil {
		return serviceBuildInfo{}, err
	}
	return build, nil
}

func launcherFilename(platform string) string {
	if platform == "windows" {
		return "gitsss-launcher.exe"
	}
	return "gitsss-launcher"
}
