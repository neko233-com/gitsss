// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	checkOnly := flag.Bool("check-only", false, "validate the output path without creating or removing files")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: releaseoutput [--check-only] OUTPUT_DIR")
		os.Exit(2)
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read repository directory: %v\n", err)
		os.Exit(1)
	}
	count, target, err := prepareOutput(root, flag.Arg(0), *checkOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "release output safety check failed: %v\n", err)
		os.Exit(1)
	}

	action := "cleared"
	if *checkOnly {
		action = "would clear"
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot display release output path: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "%s %d prior release artifacts in %s\n", action, count, relative)
}
