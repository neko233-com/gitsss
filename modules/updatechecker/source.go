// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultGitHubRepo is the release source checked by the CLI and admin UI.
	DefaultGitHubRepo = "neko233-com/gitsss"
	// DefaultMirror is a public prefix proxy that stays reachable from many
	// networks in mainland China when api.github.com does not. Replace it
	// with an internal mirror when one exists.
	DefaultMirror          = "https://ghfast.top"
	defaultProbeTimeout    = 4 * time.Second
	defaultEndpointTimeout = 10 * time.Second
	defaultDownloadTimeout = 10 * time.Minute
	maxManifestSize        = 1024 * 1024
	maxPackageSize         = 1 << 30
	maxMirrors             = 16
	kindGitHub             = "github"
	kindMirror             = "mirror"
	kindEndpoint           = "endpoint"
)

var (
	errNoSource         = errors.New("no update source configured")
	errChecksumMissing  = errors.New("release asset has no sha256 checksum")
	errChecksumMismatch = errors.New("checksum mismatch")
	errPackageTooLarge  = errors.New("update package exceeds 1 GiB")
)

// Options selects where a check looks for the latest gitsss release.
// An empty mirror list uses DefaultMirror when GitHub checking is enabled.
// Direct api.github.com is always probed alongside those mirrors.
type Options struct {
	HTTPEndpoint    string
	CheckGitHub     bool
	GitHubRepo      string
	Mirrors         []string
	ProbeTimeout    time.Duration
	DownloadTimeout time.Duration
	CurrentVersion  string
}

func (o Options) normalized() (Options, error) {
	o.HTTPEndpoint = strings.TrimSpace(o.HTTPEndpoint)
	o.GitHubRepo = strings.Trim(strings.TrimSpace(o.GitHubRepo), "/")
	if o.CheckGitHub {
		if o.GitHubRepo == "" {
			o.GitHubRepo = DefaultGitHubRepo
		}
		if err := validateRepo(o.GitHubRepo); err != nil {
			return Options{}, err
		}
		if len(o.Mirrors) == 0 {
			o.Mirrors = []string{DefaultMirror}
		}
	}
	if len(o.Mirrors) > maxMirrors {
		return Options{}, fmt.Errorf("at most %d update mirrors are allowed", maxMirrors)
	}
	cleaned := make([]string, 0, len(o.Mirrors))
	seen := make(map[string]struct{}, len(o.Mirrors))
	for _, mirror := range o.Mirrors {
		mirror = strings.TrimSpace(mirror)
		if mirror == "" {
			continue
		}
		if _, err := joinMirror(mirror, "https://api.github.com/"); err != nil {
			return Options{}, err
		}
		if _, ok := seen[mirror]; ok {
			continue
		}
		seen[mirror] = struct{}{}
		cleaned = append(cleaned, mirror)
	}
	o.Mirrors = cleaned
	if !o.CheckGitHub && o.HTTPEndpoint == "" {
		return Options{}, errNoSource
	}
	if o.HTTPEndpoint != "" {
		if err := validateHTTPURL(o.HTTPEndpoint); err != nil {
			return Options{}, fmt.Errorf("invalid update endpoint: %w", err)
		}
	}
	if o.ProbeTimeout <= 0 {
		if o.CheckGitHub {
			o.ProbeTimeout = defaultProbeTimeout
		} else {
			o.ProbeTimeout = defaultEndpointTimeout
		}
	}
	if o.DownloadTimeout <= 0 {
		o.DownloadTimeout = defaultDownloadTimeout
	}
	return o, nil
}

// ParseMirrors splits a comma, semicolon, or newline separated mirror list.
func ParseMirrors(text string) ([]string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, ";", "\n")
	text = strings.ReplaceAll(text, ",", "\n")
	var mirrors []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		mirrors = append(mirrors, line)
	}
	return mirrors, nil
}

func validateRepo(repo string) error {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || strings.Contains(name, "/") || !isRepoPart(owner) || !isRepoPart(name) {
		return fmt.Errorf("invalid GitHub repository %q", repo)
	}
	return nil
}

func isRepoPart(part string) bool {
	if part == "" || part == "." || part == ".." {
		return false
	}
	for _, r := range part {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("unsupported URL %q", raw)
	}
	return nil
}

func joinMirror(mirror, upstream string) (string, error) {
	mirror = strings.TrimSpace(mirror)
	if err := validateHTTPURL(mirror); err != nil {
		return "", fmt.Errorf("invalid update mirror %q: %w", mirror, err)
	}
	if strings.Contains(mirror, "{url}") {
		return strings.ReplaceAll(mirror, "{url}", upstream), nil
	}
	return strings.TrimRight(mirror, "/") + "/" + upstream, nil
}

func gitHubLatestURL(repo string) string {
	return "https://api.github.com/repos/" + repo + "/releases/latest"
}

type candidate struct {
	Name   string
	Kind   string
	Mirror string
	URL    string
}

func buildCandidates(opts Options) ([]candidate, error) {
	var candidates []candidate
	if opts.HTTPEndpoint != "" {
		parsed, _ := url.Parse(opts.HTTPEndpoint)
		candidates = append(candidates, candidate{
			Name: parsed.Host,
			Kind: kindEndpoint,
			URL:  opts.HTTPEndpoint,
		})
	}
	if !opts.CheckGitHub {
		return candidates, nil
	}
	direct := gitHubLatestURL(opts.GitHubRepo)
	candidates = append(candidates, candidate{
		Name: "github",
		Kind: kindGitHub,
		URL:  direct,
	})
	for _, mirror := range opts.Mirrors {
		rewritten, err := joinMirror(mirror, direct)
		if err != nil {
			return nil, err
		}
		parsed, _ := url.Parse(mirror)
		candidates = append(candidates, candidate{
			Name:   parsed.Host,
			Kind:   kindMirror,
			Mirror: mirror,
			URL:    rewritten,
		})
	}
	return candidates, nil
}

func isGitHubAssetHost(host string) bool {
	host = strings.ToLower(host)
	switch host {
	case "github.com", "api.github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com", "github-releases.githubusercontent.com":
		return true
	default:
		return strings.HasSuffix(host, ".githubusercontent.com")
	}
}

// rewriteAssetURL returns the URL to download an asset from this candidate.
// Only GitHub-hosted URLs are sent through a mirror. Company manifest assets
// stay on their original host.
func (c candidate) rewriteAssetURL(assetURL string) (string, error) {
	if err := validateHTTPURL(assetURL); err != nil {
		return "", err
	}
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return "", err
	}
	if c.Kind != kindMirror || !isGitHubAssetHost(parsed.Host) {
		return assetURL, nil
	}
	return joinMirror(c.Mirror, assetURL)
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = url.User("redacted")
	return parsed.String()
}
