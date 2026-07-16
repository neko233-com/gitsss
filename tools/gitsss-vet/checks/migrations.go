// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package checks

import (
	"errors"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Migrations = &analysis.Analyzer{
	Name: "migrations",
	Doc:  "check migrations for black-listed packages.",
	Run:  checkMigrations,
}

var (
	migrationDepBlockList = []string{
		"github.com/neko233-com/gitsss/models",
	}
	migrationImpBlockList = []string{
		"github.com/neko233-com/gitsss/modules/structs",
	}
)

func checkMigrations(pass *analysis.Pass) (interface{}, error) {
	if !strings.EqualFold(pass.Pkg.Path(), "github.com/neko233-com/gitsss/models/migrations") {
		return nil, nil
	}

	if _, err := exec.LookPath("go"); err != nil {
		return nil, errors.New("go was not found in the PATH")
	}

	depsCmd := exec.Command("go", "list", "-f", `{{join .Deps "\n"}}`, "github.com/neko233-com/gitsss/models/migrations")
	depsOut, err := depsCmd.Output()
	if err != nil {
		return nil, err
	}

	deps := strings.Split(string(depsOut), "\n")
	for _, dep := range deps {
		if stringInSlice(dep, migrationDepBlockList) {
			pass.Reportf(0, "github.com/neko233-com/gitsss/models/migrations cannot depend on the following packages: %s", migrationDepBlockList)
			return nil, nil
		}
	}

	impsCmd := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, "github.com/neko233-com/gitsss/models/migrations")
	impsOut, err := impsCmd.Output()
	if err != nil {
		return nil, err
	}

	imps := strings.Split(string(impsOut), "\n")
	for _, imp := range imps {
		if stringInSlice(imp, migrationImpBlockList) {
			pass.Reportf(0, "github.com/neko233-com/gitsss/models/migrations cannot import the following packages: %s", migrationImpBlockList)
			return nil, nil
		}
	}

	return nil, nil
}

func stringInSlice(needle string, haystack []string) bool {
	for _, h := range haystack {
		if strings.EqualFold(needle, h) {
			return true
		}
	}
	return false
}
