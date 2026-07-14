// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"os"
	"runtime"
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
