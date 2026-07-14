// Copyright 2026 gitsss Authors. All rights reserved.

package autostart

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/launcher"
)

func TestBuildPlanRendersCrossPlatformDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		options    Options
		definition string
		command    string
	}{
		{
			name: "windows",
			options: Options{
				Platform: PlatformWindows, Executable: `C:\Program Files\gitsss\gitsss-agent.exe`,
				ServerURL: "https://gitsss.internal", AgentID: "studio-agent", TokenFile: `C:\Program Data\gitsss\agent.token`,
			},
			definition: `C:\ProgramData\gitsss\gitsss-agent-task.xml`, command: "schtasks.exe",
		},
		{
			name: "darwin",
			options: Options{
				Platform: PlatformDarwin, Executable: "/usr/local/bin/gitsss-agent",
				ServerURL: "https://gitsss.internal", AgentID: "studio-agent",
			},
			definition: "/Library/LaunchDaemons/com.gitsss.agent.plist", command: "/bin/launchctl",
		},
		{
			name: "linux",
			options: Options{
				Platform: PlatformLinux, Executable: "/usr/local/bin/gitsss-agent",
				ServerURL: "https://gitsss.internal", AgentID: "studio-agent",
			},
			definition: "/etc/systemd/system/gitsss-agent.service", command: "systemctl",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := BuildPlan(test.options)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Platform != test.options.Platform || plan.DefinitionPath != test.definition || len(plan.Install) < 2 || plan.Install[0].Executable != test.command {
				t.Fatalf("unexpected plan: %#v", plan)
			}
			if len(plan.Directories) == 0 || !containsString(plan.Directories, targetDirectory(plan.Platform, plan.TokenFile)) {
				t.Fatalf("plan does not prepare token directory: %#v", plan.Directories)
			}
			for _, expected := range []string{test.options.Executable, test.options.ServerURL, test.options.AgentID, plan.TokenFile} {
				if !strings.Contains(plan.Definition, expected) {
					t.Fatalf("definition does not contain %q:\n%s", expected, plan.Definition)
				}
			}
			if strings.Contains(plan.Definition, "work-token-secret") {
				t.Fatal("service definition contains a token value")
			}
			if test.options.Platform != PlatformLinux {
				var document struct{ XMLName xml.Name }
				if err := xml.Unmarshal([]byte(plan.Definition), &document); err != nil {
					t.Fatalf("invalid XML: %v\n%s", err, plan.Definition)
				}
			}
		})
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestBuildPlanRejectsUnsafeInputs(t *testing.T) {
	base := Options{
		Platform: PlatformLinux, Executable: "/usr/local/bin/gitsss-agent",
		ServerURL: "https://gitsss.internal", AgentID: "studio-agent",
	}
	tests := []Options{
		{Platform: "freebsd", Executable: base.Executable, ServerURL: base.ServerURL, AgentID: base.AgentID},
		{Platform: base.Platform, Executable: "relative-agent", ServerURL: base.ServerURL, AgentID: base.AgentID},
		{Platform: base.Platform, Executable: base.Executable, ServerURL: "https://user:secret@gitsss.internal", AgentID: base.AgentID},
		{Platform: base.Platform, Executable: base.Executable, ServerURL: "https://gitsss.internal?token=secret", AgentID: base.AgentID},
		{Platform: base.Platform, Executable: base.Executable, ServerURL: base.ServerURL, AgentID: "unsafe agent"},
		{Platform: base.Platform, Executable: base.Executable, ServerURL: base.ServerURL, AgentID: base.AgentID, TokenFile: "relative.token"},
	}
	for index, options := range tests {
		if _, err := BuildPlan(options); err == nil {
			t.Fatalf("unsafe options %d were accepted: %#v", index, options)
		}
	}
}

func TestBuildPlanCreatesSeparateBundledRunnerService(t *testing.T) {
	plan, err := BuildPlan(Options{
		Platform: PlatformLinux, Executable: "/usr/local/bin/gitsss-agent", ServerURL: "https://gitsss.internal",
		AgentID: "runner-1", RunnerMode: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.DefinitionPath != "/etc/systemd/system/gitsss-runner.service" || plan.TokenFile != "/etc/gitsss/runner.token" ||
		!strings.Contains(plan.Definition, "pipeline-work") || !strings.Contains(plan.Definition, "--workspace") ||
		plan.Install[1].Arguments[2] != "gitsss-runner.service" {
		t.Fatalf("runner service plan = %#v\n%s", plan, plan.Definition)
	}
}

func TestWindowsAndSystemdArgumentQuoting(t *testing.T) {
	if quoted := quoteWindowsArgument(`C:\Program Files\gitsss\`); quoted != `"C:\Program Files\gitsss\\"` {
		t.Fatalf("Windows argument = %q", quoted)
	}
	if quoted := quoteSystemdArgument(`/var/lib/gitsss/100% "safe"`); quoted != `"/var/lib/gitsss/100%% \"safe\""` {
		t.Fatalf("systemd argument = %q", quoted)
	}
}

func TestBuildPlanRoutesServiceThroughLauncher(t *testing.T) {
	plan, err := BuildPlan(Options{
		Platform:           PlatformWindows,
		Executable:         `C:\Program Files\gitsss\versions\1.2.3\gitsss-agent.exe`,
		LauncherExecutable: `C:\Program Files\gitsss\gitsss-launcher.exe`,
		ServerURL:          "https://gitsss.internal", AgentID: "studio-agent",
		Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567", Architecture: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Definition, `gitsss-launcher.exe`) || !strings.Contains(plan.Definition, `--state-dir`) || strings.Contains(plan.Definition, `--token-file`) {
		t.Fatalf("service does not route through launcher:\n%s", plan.Definition)
	}
	if plan.StatePath != `C:\ProgramData\gitsss\launcher-state\active-00000000000000000001.json` || plan.StateDefinition == "" {
		t.Fatalf("unexpected launcher state: %s %q", plan.StatePath, plan.StateDefinition)
	}
	var state launcher.State
	if err := json.Unmarshal([]byte(plan.StateDefinition), &state); err != nil {
		t.Fatal(err)
	}
	if state.Platform != "windows" || state.Architecture != "amd64" || state.Executable != "gitsss-agent.exe" || !strings.Contains(plan.StateDefinition, `--token-file`) {
		t.Fatalf("unexpected state: %#v", state)
	}
}
