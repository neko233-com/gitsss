// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/glob"
)

// PackageMirror adds a disposable upstream cache without changing published packages.
var PackageMirror struct {
	Enabled           bool
	MavenURL          string
	GoURL             string
	PyPIURL           string
	NpmURL            string
	CargoIndexURL     string
	CargoFilesURL     string
	NuGetURL          string
	ComposerURL       string
	ComposerFileHosts []string
	NpmFileHosts      []string
	PyPIFileHosts     []string
	GoFileHosts       []string
	PrivatePatterns   []string
	MetadataTTL       time.Duration
	ArtifactTTL       time.Duration
	Timeout           time.Duration
	MaxSize           int64
	MaxFileSize       int64
	MaxEntries        int
}

func loadPackageMirrorFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("packages.mirror")
	// Do not inherit packages.ENABLED: central access needs its own opt-in.
	PackageMirror.Enabled = ConfigSectionKeyBool(sec, "ENABLED")
	PackageMirror.MavenURL = sec.Key("MAVEN_URL").MustString("https://repo.maven.apache.org/maven2")
	PackageMirror.GoURL = sec.Key("GO_URL").MustString("https://proxy.golang.org")
	PackageMirror.PyPIURL = sec.Key("PYPI_URL").MustString("https://pypi.org/simple")
	PackageMirror.NpmURL = sec.Key("NPM_URL").MustString("https://registry.npmjs.org")
	PackageMirror.CargoIndexURL = sec.Key("CARGO_INDEX_URL").MustString("https://index.crates.io")
	PackageMirror.CargoFilesURL = sec.Key("CARGO_FILES_URL").MustString("https://static.crates.io/crates")
	PackageMirror.NuGetURL = sec.Key("NUGET_URL").MustString("https://api.nuget.org/v3-flatcontainer")
	PackageMirror.ComposerURL = sec.Key("COMPOSER_URL").MustString("https://repo.packagist.org")
	sec.Key("COMPOSER_FILE_HOSTS").MustString("api.github.com,codeload.github.com,github.com,gitlab.com,bitbucket.org")
	PackageMirror.ComposerFileHosts = sec.Key("COMPOSER_FILE_HOSTS").Strings(",")
	PackageMirror.NpmFileHosts = sec.Key("NPM_FILE_HOSTS").Strings(",")
	sec.Key("PYPI_FILE_HOSTS").MustString("files.pythonhosted.org")
	PackageMirror.PyPIFileHosts = sec.Key("PYPI_FILE_HOSTS").Strings(",")
	sec.Key("GO_FILE_HOSTS").MustString("storage.googleapis.com")
	PackageMirror.GoFileHosts = sec.Key("GO_FILE_HOSTS").Strings(",")
	PackageMirror.PrivatePatterns = sec.Key("PRIVATE_PATTERNS").Strings(",")
	PackageMirror.MetadataTTL = sec.Key("METADATA_TTL").MustDuration(5 * time.Minute)
	PackageMirror.ArtifactTTL = sec.Key("ARTIFACT_TTL").MustDuration(168 * time.Hour)
	PackageMirror.Timeout = sec.Key("TIMEOUT").MustDuration(2 * time.Minute)
	PackageMirror.MaxSize = sec.Key("MAX_SIZE").MustInt64(10 * 1024 * 1024 * 1024)
	PackageMirror.MaxFileSize = sec.Key("MAX_FILE_SIZE").MustInt64(512 * 1024 * 1024)
	PackageMirror.MaxEntries = sec.Key("MAX_ENTRIES").MustInt(50000)
	for name, raw := range map[string]string{"MAVEN_URL": PackageMirror.MavenURL, "GO_URL": PackageMirror.GoURL, "PYPI_URL": PackageMirror.PyPIURL, "NPM_URL": PackageMirror.NpmURL, "CARGO_INDEX_URL": PackageMirror.CargoIndexURL, "CARGO_FILES_URL": PackageMirror.CargoFilesURL, "NUGET_URL": PackageMirror.NuGetURL, "COMPOSER_URL": PackageMirror.ComposerURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("packages.mirror %s must be an HTTP(S) URL without credentials, query or fragment", name)
		}
	}
	if PackageMirror.MetadataTTL <= 0 || PackageMirror.ArtifactTTL <= 0 || PackageMirror.Timeout <= 0 || PackageMirror.MaxSize <= 0 || PackageMirror.MaxFileSize <= 0 || PackageMirror.MaxFileSize > PackageMirror.MaxSize || PackageMirror.MaxEntries <= 0 {
		return errors.New("packages.mirror durations and size limits must be positive; MAX_FILE_SIZE must not exceed MAX_SIZE")
	}
	for _, hosts := range [][]string{PackageMirror.NpmFileHosts, PackageMirror.PyPIFileHosts, PackageMirror.GoFileHosts, PackageMirror.ComposerFileHosts} {
		for _, host := range hosts {
			if host == "" || strings.ContainsAny(host, "/@?#*\\") {
				return errors.New("packages.mirror file hosts must be exact hosts, optionally with a port")
			}
		}
	}
	for _, pattern := range PackageMirror.PrivatePatterns {
		if _, err := glob.Compile(pattern); err != nil {
			return fmt.Errorf("packages.mirror PRIVATE_PATTERNS: %w", err)
		}
	}
	return nil
}
