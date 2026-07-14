// Copyright 2026 gitsss Authors. All rights reserved.

package autostart

import (
	"encoding/xml"
	"strings"
	"testing"
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

func TestWindowsAndSystemdArgumentQuoting(t *testing.T) {
	if quoted := quoteWindowsArgument(`C:\Program Files\gitsss\`); quoted != `"C:\Program Files\gitsss\\"` {
		t.Fatalf("Windows argument = %q", quoted)
	}
	if quoted := quoteSystemdArgument(`/var/lib/gitsss/100% "safe"`); quoted != `"/var/lib/gitsss/100%% \"safe\""` {
		t.Fatalf("systemd argument = %q", quoted)
	}
}
