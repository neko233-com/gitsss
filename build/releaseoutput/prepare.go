// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	releaseVersionPattern  = `[A-Za-z0-9][A-Za-z0-9.+_-]*`
	releasePlatformPattern = `(?:linux|darwin|windows|freebsd)`
	releaseArchPattern     = `(?:amd64|arm64|386)`
)

var releaseArtifactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^gitsss-src-` + releaseVersionPattern + `\.tar\.gz$`),
	regexp.MustCompile(`^gitsss-` + releaseVersionPattern + `-` + releasePlatformPattern + `-` + releaseArchPattern + `(\.exe)?$`),
	regexp.MustCompile(`^gitsss-hook-` + releaseVersionPattern + `-` + releasePlatformPattern + `-` + releaseArchPattern + `(\.exe)?$`),
	regexp.MustCompile(`^gitsss-` + releaseVersionPattern + `-gogit-windows-` + releaseArchPattern + `\.exe$`),
	regexp.MustCompile(`^gitsss-` + releaseVersionPattern + `-` + releasePlatformPattern + `-` + releaseArchPattern + `\.tar\.gz$`),
}

func prepareOutput(repoRoot, outputDir string, checkOnly bool) (int, string, error) {
	root, target, err := resolveOutputPath(repoRoot, outputDir)
	if err != nil {
		return 0, "", err
	}

	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		if checkOnly {
			return 0, target, nil
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return 0, "", fmt.Errorf("create output directory: %w", err)
		}
		_, target, err = resolveOutputPath(root, target)
		if err != nil {
			return 0, "", err
		}
		info, err = os.Lstat(target)
	}
	if err != nil {
		return 0, "", fmt.Errorf("inspect output directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return 0, "", errors.New("release output must be a real directory")
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return 0, "", fmt.Errorf("list output directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return 0, "", fmt.Errorf("refusing to remove non-file output %q", entry.Name())
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return 0, "", fmt.Errorf("inspect output %q: %w", entry.Name(), err)
		}
		if !fileInfo.Mode().IsRegular() {
			return 0, "", fmt.Errorf("refusing to remove non-file output %q", entry.Name())
		}
		if !isReleaseArtifact(entry.Name()) {
			return 0, "", fmt.Errorf("refusing to remove unrecognized output %q", entry.Name())
		}
	}

	if checkOnly {
		return len(entries), target, nil
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(target, entry.Name())); err != nil {
			return 0, "", fmt.Errorf("remove prior release artifact %q: %w", entry.Name(), err)
		}
	}
	return len(entries), target, nil
}

func resolveOutputPath(repoRoot, outputDir string) (string, string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root links: %w", err)
	}

	target := outputDir
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve output path: %w", err)
	}
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("release output must be a non-root path inside the repository")
	}

	resolved := root
	currentRelative := ""
	for component := range strings.SplitSeq(relative, string(filepath.Separator)) {
		resolved = filepath.Join(resolved, component)
		currentRelative = filepath.Join(currentRelative, component)
		info, statErr := os.Lstat(resolved)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return "", "", fmt.Errorf("inspect output path component: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("release output path cannot contain symlinks or junctions")
		}
		actual, evalErr := filepath.EvalSymlinks(resolved)
		if evalErr != nil {
			return "", "", fmt.Errorf("resolve output path component: %w", evalErr)
		}
		actualRelative, evalErr := filepath.Rel(rootReal, actual)
		if evalErr != nil || !samePath(actualRelative, currentRelative) {
			return "", "", errors.New("release output path cannot contain symlinks or junctions")
		}
	}
	return root, target, nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func isReleaseArtifact(name string) bool {
	if name == "SHA256SUMS" {
		return true
	}
	for strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".xz") {
		name = name[:strings.LastIndexByte(name, '.')]
	}
	for _, pattern := range releaseArtifactPatterns {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}
