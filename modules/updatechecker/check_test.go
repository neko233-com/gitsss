// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMirrorsAndRejectsUnsupportedScheme(t *testing.T) {
	mirrors, err := ParseMirrors(" https://one.example/git \nhttps://two.example, https://three.example/{url} ")
	require.NoError(t, err)
	require.Equal(t, []string{"https://one.example/git", "https://two.example", "https://three.example/{url}"}, mirrors)

	_, err = (&Options{CheckGitHub: true, Mirrors: []string{"file:///tmp/mirror"}}).normalized()
	require.ErrorContains(t, err, "invalid update mirror")
}

func TestCheckPrefersGitHubManifestAndFallsBackToFastestMirror(t *testing.T) {
	const repo = "neko233-com/gitsss"
	const mirror = "https://mirror.example/{url}"
	githubURL := gitHubLatestURL(repo)
	mirrorURL, err := joinMirror(mirror, githubURL)
	require.NoError(t, err)
	payload := `{"tag_name":"v1.2.3","html_url":"https://github.com/neko233-com/gitsss/releases/tag/v1.2.3","assets":[{"name":"gitsss-linux-amd64.tar.gz","browser_download_url":"https://github.com/neko233-com/gitsss/releases/download/v1.2.3/gitsss-linux-amd64.tar.gz","size":4,"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`
	mirrorPayload := `{"tag_name":"v9.9.9","assets":[]}`

	client := scriptClient(map[string]http.HandlerFunc{
		githubURL: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(40 * time.Millisecond)
			_, _ = io.WriteString(w, payload)
		},
		mirrorURL: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, mirrorPayload)
		},
	})

	report, _, err := check(context.Background(), Options{
		CheckGitHub:    true,
		GitHubRepo:     repo,
		Mirrors:        []string{mirror},
		CurrentVersion: "1.2.0",
		ProbeTimeout:   time.Second,
	}, client)
	require.NoError(t, err)
	assert.True(t, report.OK)
	assert.True(t, report.NeedUpdate)
	assert.Equal(t, "v1.2.3", report.LatestVersion)
	assert.True(t, report.ManifestTrusted)
	assert.Equal(t, kindGitHub, report.ManifestKind)
	assert.Equal(t, []string{"mirror.example", "github"}, report.DownloadOrder)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", report.Assets[0].SHA256)

	report.CurrentVersion = "v1.2.3"
	assert.False(t, newerVersion(report.CurrentVersion, report.LatestVersion))
	assert.False(t, newerVersion("dev", report.LatestVersion))
}

func TestCheckUsesFastestMirrorWhenGitHubIsDown(t *testing.T) {
	const repo = "neko233-com/gitsss"
	slow := "https://slow.example"
	fast := "https://fast.example"
	githubURL := gitHubLatestURL(repo)
	slowURL, err := joinMirror(slow, githubURL)
	require.NoError(t, err)
	fastURL, err := joinMirror(fast, githubURL)
	require.NoError(t, err)
	body := `{"tag_name":"v2.0.0","html_url":"https://github.com/neko233-com/gitsss/releases/tag/v2.0.0","assets":[{"name":"SHA256SUMS","browser_download_url":"https://github.com/neko233-com/gitsss/releases/download/v2.0.0/SHA256SUMS","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},{"name":"gitsss-linux-amd64","browser_download_url":"https://github.com/neko233-com/gitsss/releases/download/v2.0.0/gitsss-linux-amd64"}]}`

	client := scriptClient(map[string]http.HandlerFunc{
		githubURL: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		},
		slowURL: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(80 * time.Millisecond)
			_, _ = io.WriteString(w, body)
		},
		fastURL: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, body)
		},
		"https://fast.example/https://github.com/neko233-com/gitsss/releases/download/v2.0.0/SHA256SUMS": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  gitsss-linux-amd64\n")
		},
	})

	report, _, err := check(context.Background(), Options{
		CheckGitHub:    true,
		GitHubRepo:     repo,
		Mirrors:        []string{slow, fast},
		CurrentVersion: "1.0.0",
		ProbeTimeout:   time.Second,
	}, client)
	require.NoError(t, err)
	assert.False(t, report.ManifestTrusted)
	assert.Equal(t, "fast.example", report.ManifestSource)
	assert.Equal(t, "v2.0.0", report.LatestVersion)
	assert.Contains(t, report.Note, "selected mirror")
	assert.Equal(t, []string{"fast.example", "slow.example"}, report.DownloadOrder)
	var binary Asset
	for _, asset := range report.Assets {
		if asset.Name == "gitsss-linux-amd64" {
			binary = asset
		}
	}
	assert.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", binary.SHA256)
	assert.Contains(t, probeText(report, "github"), "HTTP 502")
}

func TestCheckLegacyEndpointAndEmptySource(t *testing.T) {
	_, err := Check(context.Background(), Options{})
	require.ErrorIs(t, err, errNoSource)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"latest":{"version":"1.4.0"}}`)
	}))
	t.Cleanup(server.Close)

	report, err := Check(context.Background(), Options{HTTPEndpoint: server.URL, CurrentVersion: "1.3.0"})
	require.NoError(t, err)
	assert.Equal(t, "1.4.0", report.LatestVersion)
	assert.True(t, report.ManifestTrusted)
	assert.True(t, report.NeedUpdate)
}

func TestDefaultMirrorIsProbedWithoutNetwork(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		http.Error(rec, "no", http.StatusBadGateway)
		return rec.Result(), nil
	})}
	report, _, err := check(context.Background(), Options{CheckGitHub: true, ProbeTimeout: time.Second}, client)
	require.Error(t, err)
	require.Len(t, report.Probes, 2)
	assert.Equal(t, kindGitHub, report.Probes[0].Kind)
	assert.Equal(t, DefaultMirror, report.Probes[1].Mirror)
	assert.Contains(t, report.Probes[1].URL, DefaultMirror+"/https://api.github.com/repos/"+DefaultGitHubRepo+"/releases/latest")
}

func TestDownloadRejectsTamperedMirrorAndKeepsVerifiedFile(t *testing.T) {
	const repo = "neko233-com/gitsss"
	const mirror = "https://mirror.example"
	body := []byte("gitsss")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	githubURL := gitHubLatestURL(repo)
	mirrorURL, err := joinMirror(mirror, githubURL)
	require.NoError(t, err)
	assetURL := "https://github.com/neko233-com/gitsss/releases/download/v1.2.3/gitsss-linux-amd64"
	mirroredAsset, err := joinMirror(mirror, assetURL)
	require.NoError(t, err)
	manifest := `{"tag_name":"v1.2.3","assets":[{"name":"gitsss-linux-amd64","browser_download_url":"` + assetURL + `","digest":"sha256:` + digest + `"}]}`

	client := scriptClient(map[string]http.HandlerFunc{
		githubURL: func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = io.WriteString(w, manifest)
		},
		mirrorURL: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, manifest)
		},
		mirroredAsset: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "tampered")
		},
		assetURL: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		},
	})
	dir := t.TempDir()
	result, err := download(context.Background(), Options{
		CheckGitHub:    true,
		GitHubRepo:     repo,
		Mirrors:        []string{mirror},
		CurrentVersion: "1.0.0",
		ProbeTimeout:   time.Second,
	}, "gitsss-linux-amd64", dir, client)
	require.NoError(t, err)
	assert.Equal(t, "github", result.Source)
	assert.True(t, result.Verified)
	assert.Equal(t, digest, result.SHA256)
	assert.EqualValues(t, len(body), result.Bytes)
	got, err := os.ReadFile(result.Output)
	require.NoError(t, err)
	assert.Equal(t, body, got)

	_, err = SelectAsset(result.Report.Assets, "missing", "linux", "amd64")
	require.ErrorContains(t, err, "was not published")
	_, err = safeAssetFileName("../gitsss")
	require.Error(t, err)
}

func TestSaveSettingsPreservesEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ini")
	require.NoError(t, os.WriteFile(path, []byte("[cron.update_checker]\nHTTP_ENDPOINT = http://updates.intranet.example/gitsss/version.json\nENABLED = false\n"), 0o644))
	cfg, err := setting.NewConfigProviderFromFile(path)
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()

	enabled := true
	require.NoError(t, SaveSettings(Settings{
		Mirrors:     []string{"https://mirror.example", "https://backup.example/{url}"},
		GitHubRepo:  "neko233-com/gitsss",
		CheckGitHub: true,
		Enabled:     &enabled,
	}))

	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(saved)
	assert.Contains(t, text, "HTTP_ENDPOINT")
	assert.Contains(t, text, "http://updates.intranet.example/gitsss/version.json")
	assert.Contains(t, text, "https://mirror.example,https://backup.example/{url}")
	assert.Contains(t, text, "CHECK_GITHUB")
	assert.Contains(t, text, "true")

	opts := OptionsFromConfig()
	assert.Equal(t, "http://updates.intranet.example/gitsss/version.json", opts.HTTPEndpoint)
	assert.True(t, opts.CheckGitHub)
	assert.Equal(t, []string{"https://mirror.example", "https://backup.example/{url}"}, opts.Mirrors)
}

func probeText(report Report, name string) string {
	for _, probe := range report.Probes {
		if probe.Name == name {
			return probe.Error
		}
	}
	return ""
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func scriptClient(handlers map[string]http.HandlerFunc) *http.Client {
	var mu sync.Mutex
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		handler := handlers[r.URL.String()]
		mu.Unlock()
		rec := httptest.NewRecorder()
		if handler == nil {
			http.Error(rec, "no route "+r.URL.String(), http.StatusBadGateway)
			return rec.Result(), nil
		}
		handler(rec, r)
		return rec.Result(), nil
	})}
}

func TestParseChecksumFileSingleDigest(t *testing.T) {
	sums := parseChecksumFile("gitsss-linux-amd64.sha256", []byte("ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"))
	assert.Equal(t, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", sums["gitsss-linux-amd64"])
	assert.True(t, strings.HasPrefix(sums["gitsss-linux-amd64"], "abcdef"))
}
