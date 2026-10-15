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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/proxy"
	"github.com/neko233-com/gitsss/modules/system"

	"github.com/hashicorp/go-version"
)

// Asset is one file attached to a gitsss release.
type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}

// Probe is one availability and latency sample.
type Probe struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	URL        string `json:"url"`
	Mirror     string `json:"mirror,omitempty"`
	OK         bool   `json:"ok"`
	LatencyMS  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
	Selected   bool   `json:"selected"`
}

// Report is the machine-readable result of an update check.
type Report struct {
	OK              bool     `json:"ok"`
	Error           string   `json:"error,omitempty"`
	CurrentVersion  string   `json:"current_version,omitempty"`
	LatestVersion   string   `json:"latest_version,omitempty"`
	NeedUpdate      bool     `json:"need_update"`
	HTMLURL         string   `json:"html_url,omitempty"`
	ManifestSource  string   `json:"manifest_source,omitempty"`
	ManifestKind    string   `json:"manifest_kind,omitempty"`
	ManifestTrusted bool     `json:"manifest_trusted"`
	SelectedMirror  string   `json:"selected_mirror,omitempty"`
	Note            string   `json:"note,omitempty"`
	DownloadOrder   []string `json:"download_order,omitempty"`
	Assets          []Asset  `json:"assets,omitempty"`
	Probes          []Probe  `json:"probes"`
	CheckedAt       string   `json:"checked_at"`
}

// ReportState stores the last check in the application state table.
type ReportState struct {
	Report Report `json:"report"`
}

// Name returns the app-state key for the last update report.
func (r *ReportState) Name() string {
	return "update-checker-report"
}

type probeResult struct {
	candidate
	Probe   Probe
	Body    []byte
	Release *releaseDoc
}

type releaseDoc struct {
	Version string
	HTMLURL string
	Assets  []Asset
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

// Check probes the configured sources and returns the newest release it could read.
// Direct GitHub is preferred as the manifest when it answers, even if a mirror is faster.
// Mirrors are then used, fastest first, to download assets that must match that manifest.
func Check(ctx context.Context, opts Options) (Report, error) {
	report, _, err := check(ctx, opts, nil)
	return report, err
}

func check(ctx context.Context, opts Options, client *http.Client) (Report, []probeResult, error) {
	normalized, err := opts.normalized()
	if err != nil {
		return Report{}, nil, err
	}
	candidates, err := buildCandidates(normalized)
	if err != nil {
		return Report{}, nil, err
	}
	if client == nil {
		client = newHTTPClient(normalized.ProbeTimeout)
	}
	results := probeAll(ctx, client, candidates, normalized.ProbeTimeout)
	report := Report{
		CurrentVersion: normalized.CurrentVersion,
		Probes:         make([]Probe, 0, len(results)),
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	for _, result := range results {
		report.Probes = append(report.Probes, result.Probe)
	}

	chosen, trusted := selectManifest(results)
	if chosen == nil {
		report.Error = "update check failed: " + probeFailureText(results)
		return report, results, errors.New(report.Error)
	}
	// A missing checksum file does not hide the version. Download still refuses
	// an asset that has no sha256 from the manifest digest or a checksum file.
	if err := fillChecksums(ctx, client, *chosen, chosen.Release); err != nil && !errors.Is(err, errChecksumFileUnavailable) {
		report.Note = err.Error()
	}
	for i := range report.Probes {
		report.Probes[i].Selected = report.Probes[i].URL == redactURL(chosen.URL) && report.Probes[i].Kind == chosen.Kind
	}
	report.OK = true
	report.LatestVersion = chosen.Release.Version
	report.HTMLURL = chosen.Release.HTMLURL
	report.ManifestSource = chosen.Name
	report.ManifestKind = chosen.Kind
	report.ManifestTrusted = trusted
	report.SelectedMirror = chosen.Mirror
	report.Assets = append([]Asset(nil), chosen.Release.Assets...)
	report.NeedUpdate = newerVersion(normalized.CurrentVersion, chosen.Release.Version)
	report.DownloadOrder = downloadOrder(results)
	if !trusted {
		report.Note = strings.TrimSpace(report.Note + " GitHub did not answer; the checksum is bound to the selected mirror manifest.")
	}
	return report, results, nil
}

var errChecksumFileUnavailable = errors.New("checksum file unavailable")

func newerVersion(current, latest string) bool {
	cur, err := version.NewVersion(current)
	if err != nil {
		return false
	}
	remote, err := version.NewVersion(latest)
	if err != nil {
		return false
	}
	return cur.LessThan(remote)
}

func selectManifest(results []probeResult) (*probeResult, bool) {
	var trusted *probeResult
	var endpoint *probeResult
	var fastest *probeResult
	for i := range results {
		result := &results[i]
		if !result.Probe.OK || result.Release == nil {
			continue
		}
		switch result.Kind {
		case kindGitHub:
			trusted = result
		case kindEndpoint:
			if endpoint == nil || result.Probe.LatencyMS < endpoint.Probe.LatencyMS {
				endpoint = result
			}
		}
		if fastest == nil || result.Probe.LatencyMS < fastest.Probe.LatencyMS {
			fastest = result
		}
	}
	switch {
	case trusted != nil:
		return trusted, true
	case endpoint != nil:
		return endpoint, true
	default:
		return fastest, false
	}
}

func downloadOrder(results []probeResult) []string {
	healthy := make([]probeResult, 0, len(results))
	for _, result := range results {
		if result.Probe.OK {
			healthy = append(healthy, result)
		}
	}
	sort.SliceStable(healthy, func(i, j int) bool {
		return healthy[i].Probe.LatencyMS < healthy[j].Probe.LatencyMS
	})
	order := make([]string, 0, len(healthy))
	for _, result := range healthy {
		order = append(order, result.Name)
	}
	return order
}

func probeFailureText(results []probeResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		if result.Probe.OK {
			continue
		}
		message := result.Probe.Error
		if message == "" {
			message = "unavailable"
		}
		parts = append(parts, result.Name+": "+message)
	}
	if len(parts) == 0 {
		return "no reachable update source"
	}
	return strings.Join(parts, "; ")
}

func probeAll(ctx context.Context, client *http.Client, candidates []candidate, timeout time.Duration) []probeResult {
	results := make([]probeResult, len(candidates))
	var wg sync.WaitGroup
	for i, item := range candidates {
		wg.Add(1)
		go func(i int, item candidate) {
			defer wg.Done()
			results[i] = probeOne(ctx, client, item, timeout)
		}(i, item)
	}
	wg.Wait()
	return results
}

func probeOne(ctx context.Context, client *http.Client, item candidate, timeout time.Duration) probeResult {
	result := probeResult{candidate: item, Probe: Probe{
		Name:   item.Name,
		Kind:   item.Kind,
		URL:    redactURL(item.URL),
		Mirror: item.Mirror,
	}}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	started := time.Now()
	body, status, err := getLimited(probeCtx, client, item.URL, maxManifestSize)
	result.Probe.LatencyMS = time.Since(started).Milliseconds()
	result.Probe.StatusCode = status
	if err != nil {
		result.Probe.Error = err.Error()
		return result
	}
	release, err := parseManifest(body)
	if err != nil {
		result.Probe.Error = err.Error()
		return result
	}
	result.Probe.OK = true
	result.Body = body
	result.Release = &release
	return result
}

func getLimited(ctx context.Context, client *http.Client, rawURL string, limit int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json, application/json")
	req.Header.Set("User-Agent", "gitsss-update-checker")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if int64(len(body)) > limit {
		return nil, resp.StatusCode, fmt.Errorf("update response exceeds %d bytes", limit)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, resp.StatusCode, fmt.Errorf("update manifest returned HTTP %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

func parseManifest(body []byte) (releaseDoc, error) {
	var github githubRelease
	githubErr := json.Unmarshal(body, &github)
	if githubErr == nil && strings.TrimSpace(github.TagName) != "" {
		release := releaseDoc{
			Version: strings.TrimSpace(github.TagName),
			HTMLURL: github.HTMLURL,
		}
		for _, asset := range github.Assets {
			release.Assets = append(release.Assets, Asset{
				Name:   asset.Name,
				URL:    asset.BrowserDownloadURL,
				Size:   asset.Size,
				SHA256: parseDigest(asset.Digest),
			})
		}
		return release, nil
	}
	var simple struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
	}
	if err := json.Unmarshal(body, &simple); err != nil {
		if githubErr != nil {
			return releaseDoc{}, githubErr
		}
		return releaseDoc{}, err
	}
	versionText := strings.TrimSpace(simple.Latest.Version)
	if versionText == "" {
		return releaseDoc{}, errors.New("update manifest has no latest version")
	}
	return releaseDoc{Version: versionText}, nil
}

func parseDigest(digest string) string {
	algo, sum, ok := strings.Cut(strings.TrimSpace(digest), ":")
	if !ok || !strings.EqualFold(algo, "sha256") {
		return ""
	}
	sum = strings.ToLower(strings.TrimSpace(sum))
	if !isSHA256Hex(sum) {
		return ""
	}
	return sum
}

func isSHA256Hex(sum string) bool {
	if len(sum) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(sum)
	return err == nil
}

func fillChecksums(ctx context.Context, client *http.Client, source probeResult, release *releaseDoc) error {
	if release == nil || len(release.Assets) == 0 {
		return nil
	}
	var checksumAsset *Asset
	for i := range release.Assets {
		if isChecksumAsset(release.Assets[i].Name) {
			checksumAsset = &release.Assets[i]
			break
		}
	}
	if checksumAsset == nil {
		return errChecksumFileUnavailable
	}
	rawURL, err := source.rewriteAssetURL(checksumAsset.URL)
	if err != nil {
		return err
	}
	body, _, err := getLimited(ctx, client, rawURL, maxManifestSize)
	if err != nil {
		return fmt.Errorf("%w: %v", errChecksumFileUnavailable, err)
	}
	sums := parseChecksumFile(checksumAsset.Name, body)
	for i := range release.Assets {
		if release.Assets[i].SHA256 != "" {
			continue
		}
		if sum, ok := sums[release.Assets[i].Name]; ok {
			release.Assets[i].SHA256 = sum
		}
	}
	return nil
}

func isChecksumAsset(name string) bool {
	switch strings.ToLower(name) {
	case "sha256sums", "sha256sums.txt", "checksums.txt", "gitsss_checksums.txt", "gitsss-checksums.txt":
		return true
	default:
		return strings.HasSuffix(strings.ToLower(name), ".sha256")
	}
}

func parseChecksumFile(name string, body []byte) map[string]string {
	sums := make(map[string]string)
	text := strings.TrimSpace(string(body))
	if strings.HasSuffix(strings.ToLower(name), ".sha256") && !strings.Contains(text, "\n") && isSHA256Hex(strings.ToLower(text)) {
		assetName := strings.TrimSuffix(name, ".sha256")
		assetName = strings.TrimSuffix(assetName, ".SHA256")
		sums[assetName] = strings.ToLower(text)
		return sums
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !isSHA256Hex(strings.ToLower(fields[0])) {
			continue
		}
		filename := fields[len(fields)-1]
		filename = strings.TrimPrefix(filename, "*")
		sums[filename] = strings.ToLower(fields[0])
	}
	return sums
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: proxy.Proxy(),
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect to unsupported scheme")
			}
			return nil
		},
	}
}

// SaveReport stores the latest check for the admin page.
func SaveReport(ctx context.Context, report Report) error {
	if system.AppState == nil {
		return nil
	}
	return system.AppState.Set(ctx, &ReportState{Report: report})
}

// LoadReport returns the last stored check, if one exists.
func LoadReport(ctx context.Context) (Report, bool) {
	if system.AppState == nil {
		return Report{}, false
	}
	item := &ReportState{}
	if err := system.AppState.Get(ctx, item); err != nil {
		return Report{}, false
	}
	if item.Report.CheckedAt == "" && item.Report.LatestVersion == "" && len(item.Report.Probes) == 0 {
		return Report{}, false
	}
	return item.Report, true
}
