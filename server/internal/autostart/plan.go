// Copyright 2026 gitsss Authors. All rights reserved.

package autostart

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

var ErrInvalidPlan = errors.New("invalid autostart plan")

type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformDarwin  Platform = "darwin"
	PlatformLinux   Platform = "linux"
)

type Options struct {
	Platform         Platform `json:"platform"`
	Executable       string   `json:"executable"`
	ServerURL        string   `json:"server_url"`
	AgentID          string   `json:"agent_id"`
	TokenFile        string   `json:"token_file,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	LogDirectory     string   `json:"log_directory,omitempty"`
}

type Command struct {
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}

type Plan struct {
	Platform       Platform  `json:"platform"`
	DefinitionPath string    `json:"definition_path"`
	TokenFile      string    `json:"token_file"`
	Directories    []string  `json:"directories"`
	Definition     string    `json:"definition"`
	Install        []Command `json:"install"`
	Uninstall      []Command `json:"uninstall"`
}

func BuildPlan(options Options) (Plan, error) {
	if options.Platform != PlatformWindows && options.Platform != PlatformDarwin && options.Platform != PlatformLinux {
		return Plan{}, fmt.Errorf("%w: unsupported platform", ErrInvalidPlan)
	}
	if !validTargetAbsolutePath(options.Platform, options.Executable) || !validAgentID(options.AgentID) {
		return Plan{}, fmt.Errorf("%w: absolute executable and safe Agent ID are required", ErrInvalidPlan)
	}
	parsedServer, err := url.Parse(strings.TrimSpace(options.ServerURL))
	if err != nil || (parsedServer.Scheme != "http" && parsedServer.Scheme != "https") || parsedServer.Host == "" || parsedServer.User != nil || parsedServer.RawQuery != "" || parsedServer.Fragment != "" {
		return Plan{}, fmt.Errorf("%w: server URL must be a plain HTTP(S) origin or base path", ErrInvalidPlan)
	}
	options.ServerURL = strings.TrimRight(parsedServer.String(), "/")
	applyDefaults(&options)
	for _, candidate := range []string{options.TokenFile, options.WorkingDirectory} {
		if !validTargetAbsolutePath(options.Platform, candidate) {
			return Plan{}, fmt.Errorf("%w: service paths must be absolute", ErrInvalidPlan)
		}
	}
	if options.LogDirectory != "" && !validTargetAbsolutePath(options.Platform, options.LogDirectory) {
		return Plan{}, fmt.Errorf("%w: log directory must be absolute", ErrInvalidPlan)
	}
	switch options.Platform {
	case PlatformWindows:
		return windowsPlan(options), nil
	case PlatformDarwin:
		return darwinPlan(options), nil
	default:
		return linuxPlan(options), nil
	}
}

func applyDefaults(options *Options) {
	switch options.Platform {
	case PlatformWindows:
		if options.TokenFile == "" {
			options.TokenFile = `C:\ProgramData\gitsss\agent.token`
		}
		if options.WorkingDirectory == "" {
			options.WorkingDirectory = `C:\ProgramData\gitsss`
		}
	case PlatformDarwin:
		if options.TokenFile == "" {
			options.TokenFile = "/Library/Application Support/gitsss/agent.token"
		}
		if options.WorkingDirectory == "" {
			options.WorkingDirectory = "/Library/Application Support/gitsss"
		}
		if options.LogDirectory == "" {
			options.LogDirectory = "/Library/Logs/gitsss"
		}
	case PlatformLinux:
		if options.TokenFile == "" {
			options.TokenFile = "/etc/gitsss/agent.token"
		}
		if options.WorkingDirectory == "" {
			options.WorkingDirectory = "/var/lib/gitsss"
		}
	}
}

func workerArguments(options Options) []string {
	return []string{"work", "--server", options.ServerURL, "--id", options.AgentID, "--token-file", options.TokenFile}
}

func windowsPlan(options Options) Plan {
	definitionPath := `C:\ProgramData\gitsss\gitsss-agent-task.xml`
	arguments := windowsCommandLine(workerArguments(options))
	definition := `<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Author>gitsss</Author><Description>gitsss persistent Agent worker</Description></RegistrationInfo>
  <Triggers><BootTrigger><Enabled>true</Enabled></BootTrigger></Triggers>
  <Principals><Principal id="System"><UserId>S-1-5-18</UserId><LogonType>ServiceAccount</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><StartWhenAvailable>true</StartWhenAvailable><RunOnlyIfNetworkAvailable>true</RunOnlyIfNetworkAvailable><ExecutionTimeLimit>PT0S</ExecutionTimeLimit><RestartOnFailure><Interval>PT1M</Interval><Count>999</Count></RestartOnFailure></Settings>
  <Actions Context="System"><Exec><Command>` + escapeXML(options.Executable) + `</Command><Arguments>` + escapeXML(arguments) + `</Arguments><WorkingDirectory>` + escapeXML(options.WorkingDirectory) + `</WorkingDirectory></Exec></Actions>
</Task>
`
	return Plan{
		Platform: options.Platform, DefinitionPath: definitionPath, TokenFile: options.TokenFile,
		Directories: serviceDirectories(options), Definition: definition,
		Install: []Command{
			{Executable: "schtasks.exe", Arguments: []string{"/Create", "/TN", "gitsss-agent", "/XML", definitionPath, "/F"}},
			{Executable: "schtasks.exe", Arguments: []string{"/Run", "/TN", "gitsss-agent"}},
		},
		Uninstall: []Command{
			{Executable: "schtasks.exe", Arguments: []string{"/End", "/TN", "gitsss-agent"}},
			{Executable: "schtasks.exe", Arguments: []string{"/Delete", "/TN", "gitsss-agent", "/F"}},
		},
	}
}

func darwinPlan(options Options) Plan {
	definitionPath := "/Library/LaunchDaemons/com.gitsss.agent.plist"
	arguments := append([]string{options.Executable}, workerArguments(options)...)
	var argumentXML strings.Builder
	for _, argument := range arguments {
		argumentXML.WriteString("    <string>")
		argumentXML.WriteString(escapeXML(argument))
		argumentXML.WriteString("</string>\n")
	}
	definition := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.gitsss.agent</string>
  <key>ProgramArguments</key>
  <array>
` + argumentXML.String() + `  </array>
  <key>WorkingDirectory</key><string>` + escapeXML(options.WorkingDirectory) + `</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/><key>NetworkState</key><true/></dict>
  <key>ThrottleInterval</key><integer>10</integer>
  <key>StandardOutPath</key><string>` + escapeXML(path.Join(options.LogDirectory, "agent.log")) + `</string>
  <key>StandardErrorPath</key><string>` + escapeXML(path.Join(options.LogDirectory, "agent-error.log")) + `</string>
</dict>
</plist>
`
	return Plan{
		Platform: options.Platform, DefinitionPath: definitionPath, TokenFile: options.TokenFile,
		Directories: serviceDirectories(options), Definition: definition,
		Install: []Command{
			{Executable: "/bin/launchctl", Arguments: []string{"bootstrap", "system", definitionPath}},
			{Executable: "/bin/launchctl", Arguments: []string{"enable", "system/com.gitsss.agent"}},
			{Executable: "/bin/launchctl", Arguments: []string{"kickstart", "-k", "system/com.gitsss.agent"}},
		},
		Uninstall: []Command{{Executable: "/bin/launchctl", Arguments: []string{"bootout", "system/com.gitsss.agent"}}},
	}
}

func linuxPlan(options Options) Plan {
	definitionPath := "/etc/systemd/system/gitsss-agent.service"
	arguments := append([]string{options.Executable}, workerArguments(options)...)
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = quoteSystemdArgument(argument)
	}
	definition := `[Unit]
Description=gitsss persistent Agent worker
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=` + quoteSystemdArgument(options.WorkingDirectory) + `
ExecStart=` + strings.Join(quoted, " ") + `
Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=` + quoteSystemdArgument(options.WorkingDirectory) + `

[Install]
WantedBy=multi-user.target
`
	return Plan{
		Platform: options.Platform, DefinitionPath: definitionPath, TokenFile: options.TokenFile,
		Directories: serviceDirectories(options), Definition: definition,
		Install: []Command{
			{Executable: "systemctl", Arguments: []string{"daemon-reload"}},
			{Executable: "systemctl", Arguments: []string{"enable", "--now", "gitsss-agent.service"}},
		},
		Uninstall: []Command{
			{Executable: "systemctl", Arguments: []string{"disable", "--now", "gitsss-agent.service"}},
			{Executable: "systemctl", Arguments: []string{"daemon-reload"}},
		},
	}
}

func serviceDirectories(options Options) []string {
	candidates := []string{options.WorkingDirectory, targetDirectory(options.Platform, options.TokenFile)}
	if options.LogDirectory != "" {
		candidates = append(candidates, options.LogDirectory)
	}
	directories := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; candidate == "" || exists {
			continue
		}
		seen[candidate] = struct{}{}
		directories = append(directories, candidate)
	}
	return directories
}

func targetDirectory(platform Platform, filePath string) string {
	if platform != PlatformWindows {
		return path.Dir(filePath)
	}
	index := strings.LastIndexAny(filePath, `\\/`)
	if index < 0 {
		return ""
	}
	if index == 2 && len(filePath) >= 3 && filePath[1] == ':' {
		return filePath[:3]
	}
	return filePath[:index]
}

func validAgentID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validTargetAbsolutePath(platform Platform, value string) bool {
	if value == "" || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	if platform == PlatformWindows {
		return (len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')) || strings.HasPrefix(value, `\\`)
	}
	return strings.HasPrefix(value, "/") && path.Clean(value) == value
}

func escapeXML(value string) string {
	var result bytes.Buffer
	_ = xml.EscapeText(&result, []byte(value))
	return result.String()
}

func windowsCommandLine(arguments []string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = quoteWindowsArgument(argument)
	}
	return strings.Join(quoted, " ")
}

func quoteWindowsArgument(argument string) string {
	if argument != "" && !strings.ContainsAny(argument, " \t\"") {
		return argument
	}
	var result strings.Builder
	result.WriteByte('"')
	backslashes := 0
	for _, character := range argument {
		if character == '\\' {
			backslashes++
			continue
		}
		if character == '"' {
			result.WriteString(strings.Repeat("\\", backslashes*2+1))
			result.WriteRune(character)
			backslashes = 0
			continue
		}
		result.WriteString(strings.Repeat("\\", backslashes))
		backslashes = 0
		result.WriteRune(character)
	}
	result.WriteString(strings.Repeat("\\", backslashes*2))
	result.WriteByte('"')
	return result.String()
}

func quoteSystemdArgument(argument string) string {
	argument = strings.ReplaceAll(argument, "\\", "\\\\")
	argument = strings.ReplaceAll(argument, "\"", "\\\"")
	argument = strings.ReplaceAll(argument, "%", "%%")
	return `"` + argument + `"`
}
