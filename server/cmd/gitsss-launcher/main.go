// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/neko233-com/gitsss/server/internal/launcher"
)

func main() {
	flags := flag.NewFlagSet("gitsss-launcher", flag.ExitOnError)
	stateDirectory := flags.String("state-dir", "", "absolute directory containing immutable active state files")
	shutdownGrace := flags.Duration("shutdown-grace", 10*time.Second, "time allowed for the Agent to stop before it is killed")
	flags.Parse(os.Args[1:])
	if !filepath.IsAbs(*stateDirectory) || *shutdownGrace <= 0 || *shutdownGrace > time.Minute {
		fmt.Fprintln(os.Stderr, "--state-dir and a --shutdown-grace up to 1m are required")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runLauncher(ctx, *stateDirectory, *shutdownGrace, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() > 0 {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(1)
	}
}

func runLauncher(ctx context.Context, stateDirectory string, shutdownGrace time.Duration, stdout, stderr io.Writer) error {
	state, statePath, err := launcher.LoadLatest(stateDirectory)
	if err != nil {
		return err
	}
	executable, err := launcher.ExecutablePath(state)
	if err != nil {
		return err
	}
	command := exec.Command(executable, state.Arguments...)
	command.Dir = state.WorkingDirectory
	command.Stdout = stdout
	command.Stderr = stderr
	fmt.Fprintf(stdout, "gitsss launcher generation %d (%s) from %s\n", state.Generation, state.Version, statePath)
	if err := command.Start(); err != nil {
		return err
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		return err
	case <-ctx.Done():
		_ = command.Process.Signal(os.Interrupt)
		timer := time.NewTimer(shutdownGrace)
		defer timer.Stop()
		select {
		case <-waited:
			return nil
		case <-timer.C:
			if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return err
			}
			<-waited
			return nil
		}
	}
}
