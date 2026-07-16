// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/neko233-com/gitsss/tools/gitsss-vet/checks"

	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(
		checks.Imports,
		checks.License,
		checks.Migrations,
		checks.ModelsSession,
	)
}
