// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packagemirror

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCache(t *testing.T, server *httptest.Server) *Cache {
	t.Helper()
	return New(Config{
		Directory: t.TempDir(), Secret: "fixture-signing-key", Upstreams: map[string]string{"npm": server.URL, "pypi": server.URL + "/simple", "maven": server.URL, "go": server.URL},
		MetadataTTL: time.Minute, ArtifactTTL: time.Hour, Timeout: time.Second, MaxSize: 1024 * 1024, MaxFileSize: 64 * 1024,
	})
}

func entryBytes(t *testing.T, entry *Entry) string {
	t.Helper()
	f, reader, err := entry.Open()
	require.NoError(t, err)
	defer f.Close()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(data)
}

func TestCacheColdWarmRestartAndOffline(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Cookie"))
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.WriteString(w, "verified-package-bytes")
	}))
	t.Cleanup(upstream.Close)
	cache := testCache(t, upstream)
	for _, kind := range []string{"maven", "go", "npm", "pypi"} {
		entry, err := cache.Get(t.Context(), kind, "artifact", false)
		require.NoError(t, err)
		assert.Equal(t, "MISS", entry.State)
		assert.Equal(t, "verified-package-bytes", entryBytes(t, entry))
		entry, err = cache.Get(t.Context(), kind, "artifact", false)
		require.NoError(t, err)
		assert.Equal(t, "HIT", entry.State)
	}
	assert.EqualValues(t, 4, calls.Load())
	upstream.Close()
	restarted := New(cache.cfg)
	entry, err := restarted.Get(t.Context(), "go", "artifact", false)
	require.NoError(t, err)
	assert.Equal(t, "verified-package-bytes", entryBytes(t, entry))
	restarted.cfg.ArtifactTTL = time.Nanosecond
	entry, err = restarted.Get(t.Context(), "go", "artifact", false)
	require.NoError(t, err)
	assert.Equal(t, "STALE", entry.State)
}

func TestCacheConcurrentFillAndRange(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "0123456789")
	}))
	defer upstream.Close()
	cache := testCache(t, upstream)
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() {
			entry, err := cache.Get(t.Context(), "go", "mod/@v/v1.0.0.zip", false)
			assert.NoError(t, err)
			if err == nil {
				assert.Equal(t, "0123456789", entryBytes(t, entry))
			}
		})
	}
	group.Wait()
	assert.EqualValues(t, 1, calls.Load())
	entry, err := cache.Get(t.Context(), "go", "mod/@v/v1.0.0.zip", false)
	require.NoError(t, err)
	f, reader, err := entry.Open()
	require.NoError(t, err)
	defer f.Close()
	req := httptest.NewRequest(http.MethodGet, "/file", nil)
	req.Header.Set("Range", "bytes=3-5")
	resp := httptest.NewRecorder()
	http.ServeContent(resp, req, "", entry.Saved, reader)
	assert.Equal(t, http.StatusPartialContent, resp.Code)
	assert.Equal(t, "345", resp.Body.String())
}

func TestCacheRejectsErrorsOversizeTraversalAndRedirects(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/redirect":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
		default:
			_, _ = io.WriteString(w, strings.Repeat("x", 70*1024))
		}
	}))
	defer upstream.Close()
	cache := testCache(t, upstream)
	for _, relative := range []string{"../outside", "%2e%2e/outside", "a/../../outside", "//other-host/a", "a\\b", "a?url=evil"} {
		_, err := cache.Get(t.Context(), "go", relative, false)
		require.Error(t, err, relative)
	}
	for relative, status := range map[string]int{"missing": http.StatusNotFound, "big": http.StatusRequestEntityTooLarge, "redirect": http.StatusBadGateway} {
		_, err := cache.Get(t.Context(), "maven", relative, false)
		var upstreamError *Error
		require.ErrorAs(t, err, &upstreamError)
		assert.Equal(t, status, upstreamError.Status)
	}
	_, err := cache.GetURL(t.Context(), "npm", "http://127.0.0.1:1/private", false)
	require.Error(t, err)
	files, err := os.ReadDir(cache.cfg.Directory)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestCacheEvictsWithinCapacity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strings.Repeat("x", 2048)) }))
	defer upstream.Close()
	cache := testCache(t, upstream)
	cache.cfg.MaxSize = 2 * (headerSize + 2048)
	for _, relative := range []string{"a", "b", "c"} {
		_, err := cache.Get(t.Context(), "maven", relative, false)
		require.NoError(t, err)
	}
	files, err := os.ReadDir(cache.cfg.Directory)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	// A restart indexes existing files before making room for another entry.
	restarted := New(cache.cfg)
	old, err := restarted.Get(t.Context(), "maven", "d", false)
	require.NoError(t, err)
	files, err = os.ReadDir(cache.cfg.Directory)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.EqualValues(t, 2*(headerSize+2048), restarted.used)
	// Refreshing an existing entry replaces its accounting rather than consuming another slot.
	restarted.cfg.ArtifactTTL = time.Nanosecond
	_, err = restarted.Get(t.Context(), "maven", "d", false)
	require.NoError(t, err)
	assert.EqualValues(t, 2*(headerSize+2048), restarted.used)
	_, _, err = old.Open()
	assert.Error(t, err)
}

func TestCacheBoundsEntryCount(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "small") }))
	defer upstream.Close()
	cache := testCache(t, upstream)
	cache.cfg.MaxEntries = 2
	for _, relative := range []string{"first", "second", "third"} {
		_, err := cache.Get(t.Context(), "maven", relative, false)
		require.NoError(t, err)
	}
	files, err := os.ReadDir(cache.cfg.Directory)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Len(t, cache.files, 2)
}

func TestCacheUsesStaleOnRateLimitAndPropagatesRetry(t *testing.T) {
	var limited atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limited.Load() {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, "cached")
	}))
	defer upstream.Close()
	cache := testCache(t, upstream)
	_, err := cache.Get(t.Context(), "maven", "warm", true)
	require.NoError(t, err)
	limited.Store(true)
	cache.cfg.MetadataTTL = time.Nanosecond
	entry, err := cache.Get(t.Context(), "maven", "warm", true)
	require.NoError(t, err)
	assert.Equal(t, "STALE", entry.State)
	_, err = cache.Get(t.Context(), "maven", "cold", true)
	var rateLimit *RateLimitError
	require.ErrorAs(t, err, &rateLimit)
	assert.Equal(t, "30", rateLimit.RetryAfter)
}

func TestNpmMetadataRewritesScopedTarballsAndBindsOwner(t *testing.T) {
	var origin string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"name":"@scope/pkg","dist-tags":{"latest":"1.0.0"},"versions":{"1.0.0":{"dist":{"tarball":%q,"integrity":"sha512-fixture"},"dependencies":{"dep":"^2"}}}}`, origin+"/@scope/pkg/-/pkg-1.0.0.tgz")
	}))
	defer upstream.Close()
	origin = upstream.URL
	cache := testCache(t, upstream)
	entry, err := cache.Get(t.Context(), "npm", "@scope/pkg", true)
	require.NoError(t, err)
	data, err := cache.NpmMetadata(entry, "42", "@scope/pkg", "http://internal/api/packages/org/npm")
	require.NoError(t, err)
	var metadata struct {
		Versions map[string]struct {
			Dist struct {
				Tarball   string `json:"tarball"`
				Integrity string `json:"integrity"`
			} `json:"dist"`
			Dependencies map[string]string `json:"dependencies"`
		} `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(data, &metadata))
	version := metadata.Versions["1.0.0"]
	assert.Equal(t, "sha512-fixture", version.Dist.Integrity)
	assert.Equal(t, "^2", version.Dependencies["dep"])
	link, err := url.Parse(version.Dist.Tarball)
	require.NoError(t, err)
	assert.Equal(t, "internal", link.Host)
	parts := strings.Split(link.Path, "/")
	token := parts[len(parts)-2]
	artifact, err := cache.ResolveArtifact("npm", "42", token)
	require.NoError(t, err)
	assert.Equal(t, origin+"/@scope/pkg/-/pkg-1.0.0.tgz", artifact.URL)
	_, err = cache.ResolveArtifact("npm", "43", token)
	require.Error(t, err)
	_, err = cache.ResolveArtifact("pypi", "42", token)
	require.Error(t, err)
	_, err = cache.ResolveArtifact("npm", "42", token+"changed")
	require.Error(t, err)
}

func TestComposerMetadataRejectsUnapprovedArtifactHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"packages":{"example/library":[{"version":"1.0.0","dist":{"url":"http://127.0.0.1:1/private"}}]}}`)
	}))
	defer upstream.Close()
	cache := testCache(t, upstream)
	cache.cfg.Upstreams["composer"] = upstream.URL
	cache.clients["composer"] = cache.clients["npm"]
	entry, err := cache.Get(t.Context(), "composer", "p2/example/library.json", true)
	require.NoError(t, err)
	_, err = cache.ComposerMetadata(entry, "1", "example/library", "http://registry/composer")
	assert.Error(t, err)
}

func TestPyPIMetadataPreservesHashesAndConstraints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><script>alert(1)</script><a href="../../files/pkg.whl#sha256=abc" data-requires-python="&gt;=3.8" data-core-metadata="sha256=def" data-yanked="bad build">pkg.whl</a></html>`)
	}))
	defer upstream.Close()
	cache := testCache(t, upstream)
	entry, err := cache.Get(t.Context(), "pypi", "pkg/", true)
	require.NoError(t, err)
	data, err := cache.PyPIMetadata(entry, "42", "pkg", "http://internal/api/packages/org/pypi", "pkg/")
	require.NoError(t, err)
	assert.Contains(t, string(data), "http://internal/api/packages/org/pypi/mirror/")
	assert.Contains(t, string(data), `#sha256=abc`)
	assert.Contains(t, string(data), `data-core-metadata="sha256=def"`)
	assert.Contains(t, string(data), `data-requires-python="&gt;=3.8"`)
	assert.Contains(t, string(data), `data-yanked="bad build"`)
	assert.NotContains(t, string(data), "script")
}
