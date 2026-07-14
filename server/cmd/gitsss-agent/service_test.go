// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/autostart"
)

func TestParseServicePlanSupportsReviewAndSimpleUninstall(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := parseServicePlan("service-plan", []string{
		"--platform", runtime.GOOS, "--executable", executable,
		"--server", "https://gitsss.internal", "--id", "studio-agent",
	}, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Platform != autostart.Platform(runtime.GOOS) || len(plan.Install) == 0 {
		t.Fatalf("unexpected review plan: %#v", plan)
	}
	uninstallPlan, err := parseServicePlan("service-uninstall", nil, "http://localhost", "uninstall", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(uninstallPlan.Uninstall) == 0 || uninstallPlan.TokenFile == "" {
		t.Fatalf("unexpected uninstall plan: %#v", uninstallPlan)
	}
}

func TestParseServicePlanAutoDetectsPackagedLauncher(t *testing.T) {
	root := t.TempDir()
	agentName := "gitsss-agent"
	if runtime.GOOS == "windows" {
		agentName += ".exe"
	}
	agentPath := filepath.Join(root, agentName)
	launcherPath := filepath.Join(root, launcherFilename(runtime.GOOS))
	for _, filePath := range []string{agentPath, launcherPath} {
		if err := os.WriteFile(filePath, []byte("binary"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	buildInfo, err := json.Marshal(serviceBuildInfo{
		Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "BUILDINFO.json"), buildInfo, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := parseServicePlan("service-plan", []string{
		"--executable", agentPath, "--server", "https://gitsss.internal", "--id", "studio-agent",
	}, "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.StatePath == "" || plan.StateDefinition == "" || !strings.Contains(plan.Definition, launcherPath) {
		t.Fatalf("launcher was not auto-detected: %#v", plan)
	}
}
