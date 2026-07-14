// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/launcher"
	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

type osActivationHost struct {
	stateDirectory string
	healthTimeout  time.Duration
	restartedAt    time.Time
}

func updateActivate(args []string) {
	flags := flag.NewFlagSet("update-activate", flag.ExitOnError)
	targetPath := flags.String("target", "", "absolute verified staging directory")
	stateDirectory := flags.String("state-dir", "", "absolute immutable launcher state directory")
	healthTimeout := flags.Duration("health-timeout", 45*time.Second, "maximum time to observe launcher start and control-plane health")
	flags.Parse(args)
	if !filepath.IsAbs(*targetPath) || !filepath.IsAbs(*stateDirectory) || *healthTimeout < time.Second || *healthTimeout > 5*time.Minute {
		fmt.Fprintln(os.Stderr, "--target, --state-dir and a --health-timeout from 1s to 5m are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *healthTimeout*3)
	defer cancel()
	host := &osActivationHost{stateDirectory: *stateDirectory, healthTimeout: *healthTimeout}
	result, err := gitsssupdate.Activate(ctx, filepath.Clean(*targetPath), filepath.Clean(*stateDirectory), host)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		fmt.Fprintln(os.Stderr, encodeErr)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (host *osActivationHost) Restart(ctx context.Context, _ launcher.State) error {
	host.restartedAt = time.Now().UTC()
	switch runtime.GOOS {
	case "windows":
		_ = runActivationCommand(ctx, "schtasks.exe", "/End", "/TN", "gitsss-agent")
		return runActivationCommand(ctx, "schtasks.exe", "/Run", "/TN", "gitsss-agent")
	case "darwin":
		return runActivationCommand(ctx, "/bin/launchctl", "kickstart", "-k", "system/com.gitsss.agent")
	case "linux":
		return runActivationCommand(ctx, "systemctl", "restart", "gitsss-agent.service")
	default:
		return fmt.Errorf("unsupported activation platform %s", runtime.GOOS)
	}
}

func (host *osActivationHost) Healthy(ctx context.Context, state launcher.State) error {
	if host.restartedAt.IsZero() {
		return errors.New("service was not restarted")
	}
	healthContext, cancel := context.WithTimeout(ctx, host.healthTimeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		started, err := launcher.StartedAfter(host.stateDirectory, state.Generation, host.restartedAt)
		if err != nil {
			return fmt.Errorf("read launcher start receipt: %w", err)
		}
		if started {
			break
		}
		select {
		case <-healthContext.Done():
			return fmt.Errorf("launcher generation %d did not start: %w", state.Generation, healthContext.Err())
		case <-ticker.C:
		}
	}
	executable, err := launcher.ExecutablePath(state)
	if err != nil {
		return err
	}
	healthArguments, err := activationHealthArguments(state.Arguments)
	if err != nil {
		return err
	}
	command := exec.CommandContext(healthContext, executable, healthArguments...)
	command.Dir = state.WorkingDirectory
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("run staged Agent health: %w", err)
		}
		return fmt.Errorf("run staged Agent health: %w: %s", err, detail)
	}
	return nil
}

func activationHealthArguments(arguments []string) ([]string, error) {
	if len(arguments) < 2 || arguments[0] != "work" {
		return nil, errors.New("launcher state does not contain Agent work arguments")
	}
	values := make(map[string]string)
	for index := 1; index < len(arguments); index += 2 {
		if index+1 >= len(arguments) || (arguments[index] != "--server" && arguments[index] != "--id" && arguments[index] != "--token-file") || arguments[index+1] == "" {
			return nil, errors.New("launcher state contains unsupported Agent work arguments")
		}
		if _, exists := values[arguments[index]]; exists {
			return nil, errors.New("launcher state contains duplicate Agent work arguments")
		}
		values[arguments[index]] = arguments[index+1]
	}
	if values["--server"] == "" || values["--id"] == "" || !filepath.IsAbs(values["--token-file"]) {
		return nil, errors.New("launcher state is missing server, Agent ID or absolute token file")
	}
	return []string{"health", "--server", values["--server"], "--token-file", values["--token-file"]}, nil
}

func runActivationCommand(ctx context.Context, executable string, arguments ...string) error {
	command := exec.CommandContext(ctx, executable, arguments...)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", executable, err)
	}
	return fmt.Errorf("%s: %w: %s", executable, err, detail)
}
