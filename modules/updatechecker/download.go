// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// DownloadResult is the machine-readable result of a verified package download.
type DownloadResult struct {
	Report   Report `json:"report"`
	Asset    string `json:"asset"`
	Output   string `json:"output"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
	Verified bool   `json:"verified"`
	Source   string `json:"source"`
}

// Download fetches one release asset and refuses to keep it unless the sha256 matches
// the selected manifest. Reachable sources are tried from lowest latency to highest.
func Download(ctx context.Context, opts Options, assetName, output string) (DownloadResult, error) {
	return download(ctx, opts, assetName, output, nil)
}

func download(ctx context.Context, opts Options, assetName, output string, client *http.Client) (DownloadResult, error) {
	normalized, err := opts.normalized()
	if err != nil {
		return DownloadResult{}, err
	}
	if client == nil {
		client = newHTTPClient(normalized.DownloadTimeout)
	}
	report, results, err := check(ctx, normalized, client)
	result := DownloadResult{Report: report}
	if err != nil {
		return result, err
	}
	asset, err := SelectAsset(report.Assets, assetName, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return result, err
	}
	if asset.SHA256 == "" {
		return result, fmt.Errorf("%w: %s", errChecksumMissing, asset.Name)
	}
	dest, err := resolveOutput(output, asset.Name)
	if err != nil {
		return result, err
	}
	sources := healthyByLatency(results)
	var lastErr error
	seen := make(map[string]struct{})
	for _, source := range sources {
		rawURL, err := source.rewriteAssetURL(asset.URL)
		if err != nil {
			lastErr = err
			continue
		}
		if _, ok := seen[rawURL]; ok {
			continue
		}
		seen[rawURL] = struct{}{}
		written, err := writeVerified(ctx, client, rawURL, dest, asset.SHA256)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", source.Name, err)
			continue
		}
		result.Asset = asset.Name
		result.Output = dest
		result.SHA256 = asset.SHA256
		result.Bytes = written
		result.Verified = true
		result.Source = source.Name
		return result, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no reachable download source")
	}
	return result, lastErr
}

func healthyByLatency(results []probeResult) []probeResult {
	healthy := make([]probeResult, 0, len(results))
	for _, result := range results {
		if result.Probe.OK {
			healthy = append(healthy, result)
		}
	}
	sort.SliceStable(healthy, func(i, j int) bool {
		return healthy[i].Probe.LatencyMS < healthy[j].Probe.LatencyMS
	})
	return healthy
}

// SelectAsset picks an explicit asset name or the best match for this OS and architecture.
func SelectAsset(assets []Asset, explicit, goos, goarch string) (Asset, error) {
	if explicit != "" {
		for _, asset := range assets {
			if asset.Name == explicit {
				if _, err := safeAssetFileName(asset.Name); err != nil {
					return Asset{}, err
				}
				return asset, nil
			}
		}
		return Asset{}, fmt.Errorf("asset %q was not published in this release", explicit)
	}
	matches := make([]Asset, 0, 1)
	for _, asset := range assets {
		if isChecksumAsset(asset.Name) {
			continue
		}
		if _, err := safeAssetFileName(asset.Name); err != nil {
			continue
		}
		if assetMatchesPlatform(asset.Name, goos, goarch) {
			matches = append(matches, asset)
		}
	}
	if len(matches) == 0 {
		return Asset{}, errors.New("no release asset matches this OS and architecture; pass an asset name")
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return assetPreference(matches[i].Name) < assetPreference(matches[j].Name)
	})
	if assetPreference(matches[0].Name) == assetPreference(matches[1].Name) {
		return Asset{}, fmt.Errorf("multiple assets match this platform, pass an asset name (first candidates: %s, %s)", matches[0].Name, matches[1].Name)
	}
	return matches[0], nil
}

func assetMatchesPlatform(name, goos, goarch string) bool {
	lower := strings.ToLower(name)
	if !strings.Contains(lower, strings.ToLower(goos)) && !(goos == "darwin" && strings.Contains(lower, "macos")) {
		return false
	}
	for _, arch := range archNames(goarch) {
		if strings.Contains(lower, arch) {
			return true
		}
	}
	return false
}

func archNames(goarch string) []string {
	switch goarch {
	case "amd64":
		return []string{"amd64", "x86_64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	case "386":
		return []string{"386", "i386", "x86"}
	default:
		return []string{strings.ToLower(goarch)}
	}
}

func assetPreference(name string) int {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".zip"):
		return 0
	case strings.HasSuffix(lower, ".sha256"), strings.Contains(lower, "checksum"):
		return 3
	default:
		return 1
	}
}

func safeAssetFileName(name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid asset name %q", name)
	}
	return name, nil
}

func resolveOutput(output, assetName string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return "", errors.New("output path is required")
	}
	info, err := os.Stat(output)
	if err == nil && info.IsDir() {
		safe, err := safeAssetFileName(assetName)
		if err != nil {
			return "", err
		}
		return filepath.Join(output, safe), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return output, nil
}

func writeVerified(ctx context.Context, client *http.Client, rawURL, dest, want string) (int64, error) {
	if !isSHA256Hex(want) {
		return 0, errChecksumMissing
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "gitsss-update-checker")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("update package returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxPackageSize {
		return 0, errPackageTooLarge
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".gitsss-update-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	remove := true
	defer func() {
		_ = tmp.Close()
		if remove {
			_ = os.Remove(tmpName)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, maxPackageSize+1))
	if err != nil {
		return 0, err
	}
	if written > maxPackageSize {
		return 0, errPackageTooLarge
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, want) {
		return 0, fmt.Errorf("%w: got %s want %s", errChecksumMismatch, sum, strings.ToLower(want))
	}
	if err := tmp.Sync(); err != nil {
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return 0, err
	}
	remove = false
	return written, nil
}
