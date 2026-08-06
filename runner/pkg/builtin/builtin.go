// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package builtin runs the Actions runner in the Gitsss server process.
package builtin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/neko233-com/gitsss/runner/internal/app/poll"
	"github.com/neko233-com/gitsss/runner/internal/app/run"
	"github.com/neko233-com/gitsss/runner/internal/pkg/client"
	"github.com/neko233-com/gitsss/runner/internal/pkg/config"
	"github.com/neko233-com/gitsss/runner/internal/pkg/labels"
	"github.com/neko233-com/gitsss/runner/internal/pkg/ver"

	"connectrpc.com/connect"
	log "github.com/sirupsen/logrus"
)

const (
	defaultCapacity        = 2
	defaultShutdownTimeout = 30 * time.Second
	defaultRunnerTimeout   = 3 * time.Hour
	defaultFetchTimeout    = 5 * time.Second
	defaultFetchInterval   = 2 * time.Second
	defaultFetchMax        = 5 * time.Second
)

// Registration is the in-memory credential and identity of one server-owned runner.
// The token is never persisted by this package; the server owns its database record.
type Registration struct {
	ID        int64
	UUID      string
	Name      string
	Version   string
	Token     string
	Labels    []string
	Ephemeral bool
}

// Options controls the built-in runner without changing the external runner config file.
type Options struct {
	Address        string
	Insecure       bool
	JobInstanceURL string
	WorkdirParent  string
	CacheDir       string
	CacheHost      string
	CachePort      uint16
	Capacity       int
	Labels         []string
	Registration   Registration
}

// Handle represents a running built-in runner.
type Handle struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Close stops polling and waits for active jobs to report their final state.
func (h *Handle) Close() {
	if h == nil || h.cancel == nil {
		return
	}
	h.cancel()
	<-h.done
}

// Start starts the runner and keeps retrying declaration until the server API is ready.
func Start(ctx context.Context, opts Options) (*Handle, error) {
	if ctx == nil {
		return nil, errors.New("builtin runner context is nil")
	}
	if opts.Address == "" {
		return nil, errors.New("builtin runner address is empty")
	}
	if opts.Registration.UUID == "" || opts.Registration.Token == "" {
		return nil, errors.New("builtin runner registration is incomplete")
	}

	cfg, err := config.LoadDefault("")
	if err != nil {
		return nil, fmt.Errorf("load built-in runner defaults: %w", err)
	}
	cfg.Runner.Capacity = opts.Capacity
	if cfg.Runner.Capacity <= 0 {
		cfg.Runner.Capacity = defaultCapacity
	}
	cfg.Runner.Timeout = defaultRunnerTimeout
	cfg.Runner.ShutdownTimeout = defaultShutdownTimeout
	cfg.Runner.FetchTimeout = defaultFetchTimeout
	cfg.Runner.FetchInterval = defaultFetchInterval
	cfg.Runner.FetchIntervalMax = defaultFetchMax
	cfg.Runner.Labels = append([]string(nil), opts.Labels...)
	cfg.Runner.JobInstanceURL = opts.JobInstanceURL
	if opts.WorkdirParent != "" {
		cfg.Container.WorkdirParent = filepath.Clean(opts.WorkdirParent)
		cfg.Container.BindWorkdir = true
	}
	if opts.CacheDir != "" {
		cfg.Cache.Dir = filepath.Clean(opts.CacheDir)
	}
	if opts.CacheHost != "" {
		cfg.Cache.Host = opts.CacheHost
	}
	if opts.CachePort != 0 {
		cfg.Cache.Port = opts.CachePort
	}

	parsedLabels := labels.Labels{}
	for _, raw := range opts.Labels {
		label, err := labels.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse built-in runner label %q: %w", raw, err)
		}
		parsedLabels = append(parsedLabels, label)
	}
	if len(parsedLabels) == 0 {
		return nil, errors.New("built-in runner requires at least one label")
	}

	reg := &config.Registration{
		ID:        opts.Registration.ID,
		UUID:      opts.Registration.UUID,
		Name:      opts.Registration.Name,
		Token:     opts.Registration.Token,
		Address:   opts.Address,
		Labels:    parsedLabels.ToStrings(),
		Ephemeral: opts.Registration.Ephemeral,
	}
	cli := client.New(opts.Address, opts.Insecure, reg.UUID, reg.Token, version(opts.Registration.Version))
	runner := run.NewRunner(cfg, reg, cli)

	runCtx, cancel := context.WithCancel(ctx)
	handle := &Handle{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(handle.done)
		if err := declareUntilReady(runCtx, runner, parsedLabels.Names()); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.WithError(err).Error("built-in Actions runner stopped before declaration")
			}
			return
		}

		supervisePoller(runCtx, cfg, cli, runner)
	}()

	return handle, nil
}

func supervisePoller(ctx context.Context, cfg *config.Config, cli client.Client, runner *run.Runner) {
	for {
		poller := poll.New(cfg, cli, runner)
		pollerDone := make(chan bool, 1)
		go func() { pollerDone <- runPoller(poller) }()

		select {
		case <-ctx.Done():
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Runner.ShutdownTimeout)
			err := poller.Shutdown(shutdownCtx)
			shutdownCancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				log.WithError(err).Warn("built-in Actions runner shutdown was forced")
			}
			return
		case panicked := <-pollerDone:
			if ctx.Err() != nil {
				return
			}
			if panicked {
				log.Warn("built-in Actions runner poller recovered from panic; restarting")
			} else {
				log.Warn("built-in Actions runner poller exited; restarting")
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}
}

func runPoller(poller *poll.Poller) (panicked bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.WithField("panic", recovered).Error("panic in built-in Actions runner poller")
			panicked = true
		}
	}()
	poller.Poll()
	return false
}

func version(value string) string {
	if value != "" {
		return value
	}
	return ver.Version()
}

func declareUntilReady(ctx context.Context, runner *run.Runner, names []string) error {
	backoff := 250 * time.Millisecond
	for {
		_, err := runner.Declare(ctx, names)
		if err == nil {
			return nil
		}
		if connect.CodeOf(err) == connect.CodeUnauthenticated || connect.CodeOf(err) == connect.CodePermissionDenied {
			return fmt.Errorf("declare built-in runner: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}
