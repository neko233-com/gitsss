// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/neko233-com/gitsss/modules/hookrelay"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--protocol-version" {
		fmt.Println(hookrelay.Protocol)
		return
	}
	mode := filepath.Base(os.Args[0])
	if mode == "pre-receive" || mode == "update" || mode == "post-receive" {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		os.Exit(hookrelay.Dispatch(ctx, mode, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
	}
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if os.Args[1] == "--run-custom-hook" && len(os.Args) >= 3 {
		os.Exit(hookrelay.RunCustomHook(ctx, os.Args[2], os.Args[3:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(hookrelay.Run(ctx, os.Args[1], os.Args[2:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}
