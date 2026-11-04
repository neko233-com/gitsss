// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packagemirror

import (
	"bytes"
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/hostmatcher"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/proxy"

	"golang.org/x/sync/singleflight"
)

const headerSize = 4096

type Config struct {
	Directory, Secret string
	Upstreams         map[string]string
	FileHosts         map[string][]string
	MetadataTTL       time.Duration
	ArtifactTTL       time.Duration
	Timeout           time.Duration
	MaxSize           int64
	MaxFileSize       int64
	MaxEntries        int
}

type Cache struct {
	cfg     Config
	clients map[string]*http.Client
	group   singleflight.Group
	mu      sync.Mutex
	slots   chan struct{}
	indexed bool
	used    int64
	files   map[string]*cacheFile
	oldest  cacheHeap
}

type cacheFile struct {
	path     string
	size     int64
	modified time.Time
	index    int
}

type cacheHeap []*cacheFile

func (h cacheHeap) Len() int           { return len(h) }
func (h cacheHeap) Less(i, j int) bool { return h[i].modified.Before(h[j].modified) }
func (h cacheHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *cacheHeap) Push(x any)        { f := x.(*cacheFile); f.index = len(*h); *h = append(*h, f) }

func (h *cacheHeap) Pop() any {
	old := *h
	f := old[len(old)-1]
	old[len(old)-1] = nil
	f.index = -1
	*h = old[:len(old)-1]
	return f
}

type Entry struct {
	Path        string `json:"-"`
	ContentType string
	ETag        string
	Saved       time.Time
	Size        int64
	State       string `json:"-"`
}

// Error contains only a status; upstream URLs and response bodies are not exposed.
type Error struct{ Status int }

func (e *Error) Error() string { return fmt.Sprintf("package mirror: HTTP %d", e.Status) }

type RateLimitError struct{ RetryAfter string }

func (*RateLimitError) Error() string { return "package mirror: HTTP 429" }
func (*RateLimitError) Unwrap() error { return &Error{http.StatusTooManyRequests} }

func New(cfg Config) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 50000
	}
	c := &Cache{cfg: cfg, clients: make(map[string]*http.Client), slots: make(chan struct{}, 4)}
	for kind, raw := range cfg.Upstreams {
		base, _ := url.Parse(raw)
		hosts := []string{base.Hostname()}
		for _, host := range cfg.FileHosts[kind] {
			fileURL, _ := url.Parse("//" + host)
			hosts = append(hosts, fileURL.Hostname())
		}
		proxyURL, _ := url.Parse(proxy.GetProxyURL())
		transport := hostmatcher.NewHTTPTransport("package mirror", hostmatcher.ParseHostMatchList("packages.mirror", strings.Join(hosts, ",")), nil, proxy.Proxy(), proxyURL, nil)
		c.clients[kind] = &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 || !c.allowed(kind, req.URL) {
					return errors.New("package mirror redirect refused")
				}
				return nil
			},
		}
	}
	return c
}

func (c *Cache) allowed(kind string, u *url.URL) bool {
	base, err := url.Parse(c.cfg.Upstreams[kind])
	if err != nil || base.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || (u.Scheme != "https" && !(base.Scheme == "http" && u.Scheme == "http")) {
		return false
	}
	if strings.EqualFold(base.Host, u.Host) {
		return true
	}
	for _, host := range c.cfg.FileHosts[kind] {
		if strings.EqualFold(host, u.Host) {
			return true
		}
	}
	return false
}

func (c *Cache) UpstreamURL(kind, relative string) (string, error) {
	base, err := url.Parse(c.cfg.Upstreams[kind])
	if err != nil || base.Host == "" || relative == "" || strings.HasPrefix(relative, "/") || strings.ContainsAny(relative, "\\?#\x00\r\n") {
		return "", &Error{http.StatusBadRequest}
	}
	decoded, err := url.PathUnescape(relative)
	if err != nil {
		return "", &Error{http.StatusBadRequest}
	}
	for segment := range strings.SplitSeq(decoded, "/") {
		if segment == "." || segment == ".." || strings.ContainsAny(segment, "\\\x00\r\n") {
			return "", &Error{http.StatusBadRequest}
		}
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + relative
	base.RawPath = ""
	return base.String(), nil
}

func (c *Cache) Get(ctx context.Context, kind, relative string, metadata bool) (*Entry, error) {
	raw, err := c.UpstreamURL(kind, relative)
	if err != nil {
		return nil, err
	}
	return c.GetURL(ctx, kind, raw, metadata)
}

func (c *Cache) GetURL(ctx context.Context, kind, raw string, metadata bool) (*Entry, error) {
	u, err := url.Parse(raw)
	if err != nil || !c.allowed(kind, u) {
		return nil, &Error{http.StatusBadRequest}
	}
	sum := sha256.Sum256([]byte(kind + "\x00" + raw))
	key := hex.EncodeToString(sum[:])
	target := filepath.Join(c.cfg.Directory, key+".cache")
	ttl := c.cfg.ArtifactTTL
	if metadata {
		ttl = c.cfg.MetadataTTL
	}
	if entry, err := readEntry(target); err == nil && time.Since(entry.Saved) < ttl {
		entry.State = "HIT"
		return entry, nil
	}
	result := c.group.DoChan(key, func() (any, error) {
		if entry, err := readEntry(target); err == nil && time.Since(entry.Saved) < ttl {
			entry.State = "HIT"
			return entry, nil
		}
		// A cancelled caller must not abort the shared fill for other clients.
		fillCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.cfg.Timeout)
		defer cancel()
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
		case <-fillCtx.Done():
			return nil, &Error{http.StatusGatewayTimeout}
		}
		entry, err := c.fetch(fillCtx, kind, raw, target, metadata)
		upstreamErr, isUpstreamError := errors.AsType[*Error](err)
		if err != nil && (!isUpstreamError || upstreamErr.Status >= 500 || upstreamErr.Status == http.StatusTooManyRequests) {
			if stale, readErr := readEntry(target); readErr == nil {
				stale.State = "STALE"
				return stale, nil
			}
		}
		return entry, err
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.(*Entry), nil
	}
}

func readEntry(target string) (*Entry, error) {
	f, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entry, err := readEntryFile(f)
	if err != nil {
		return nil, err
	}
	entry.Path = target
	return entry, nil
}

func readEntryFile(f *os.File) (*Entry, error) {
	var entry Entry
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes.TrimRight(header, "\x00\n "), &entry); err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || entry.Size < 0 || info.Size() != headerSize+entry.Size {
		return nil, errors.New("invalid package mirror cache entry")
	}
	return &entry, nil
}

func (c *Cache) fetch(ctx context.Context, kind, raw, target string, metadata bool) (*Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "npm":
		req.Header.Set("Accept", "application/json")
	case "pypi":
		req.Header.Set("Accept", "text/html")
	}
	resp, err := c.clients[kind].Do(req)
	if err != nil {
		if requestErr, ok := errors.AsType[*url.Error](err); ok {
			return nil, fmt.Errorf("upstream transport: %s: %w", requestErr.Err, &Error{http.StatusBadGateway})
		}
		return nil, &Error{http.StatusBadGateway}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &RateLimitError{RetryAfter: resp.Header.Get("Retry-After")}
		}
		status := http.StatusBadGateway
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			status = resp.StatusCode
		}
		if status == http.StatusBadGateway {
			return nil, fmt.Errorf("upstream status %d: %w", resp.StatusCode, &Error{status})
		}
		return nil, &Error{status}
	}
	limit := c.cfg.MaxFileSize
	if metadata {
		limit = min(limit, maxMetadataSize)
	}
	if resp.ContentLength > limit {
		return nil, &Error{http.StatusRequestEntityTooLarge}
	}
	if err = os.MkdirAll(c.cfg.Directory, 0o700); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(c.cfg.Directory, ".fill-*.part")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if _, err = f.Seek(headerSize, io.SeekStart); err != nil {
		return nil, err
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if size > limit {
		return nil, &Error{http.StatusRequestEntityTooLarge}
	}
	if resp.ContentLength >= 0 && resp.ContentLength != size {
		return nil, &Error{http.StatusBadGateway}
	}
	if err = f.Truncate(headerSize + size); err != nil {
		return nil, err
	}
	entry := &Entry{Path: target, ContentType: resp.Header.Get("Content-Type"), ETag: `"` + hex.EncodeToString(hash.Sum(nil)) + `"`, Saved: time.Now().UTC(), Size: size, State: "MISS"}
	header, err := json.Marshal(entry)
	if err != nil || len(header)+1 > headerSize {
		return nil, errors.New("package mirror cache header too large")
	}
	if _, err = f.WriteAt(append(header, '\n'), 0); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = c.makeRoom(target, size+headerSize); err != nil {
		return nil, err
	}
	if err = os.Rename(f.Name(), target); err != nil {
		return nil, err
	}
	if old := c.files[target]; old != nil {
		heap.Remove(&c.oldest, old.index)
		c.used -= old.size
	}
	file := &cacheFile{path: target, size: size + headerSize, modified: entry.Saved}
	c.files[target] = file
	c.used += file.size
	heap.Push(&c.oldest, file)
	return entry, nil
}

func (c *Cache) makeRoom(target string, needed int64) error {
	if !c.indexed {
		if err := c.indexFiles(); err != nil {
			return err
		}
	}
	used := c.used
	count := len(c.files)
	if old := c.files[target]; old != nil {
		used -= old.size
		count--
	}
	var skipped []*cacheFile
	defer func() {
		for _, file := range skipped {
			heap.Push(&c.oldest, file)
		}
	}()
	for (used+needed > c.cfg.MaxSize || count+1 > c.cfg.MaxEntries) && c.oldest.Len() > 0 {
		file := heap.Pop(&c.oldest).(*cacheFile)
		if file.path == target {
			skipped = append(skipped, file)
			continue
		}
		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			skipped = append(skipped, file)
			continue
		}
		delete(c.files, file.path)
		c.used -= file.size
		used -= file.size
		count--
	}
	if used+needed > c.cfg.MaxSize || count+1 > c.cfg.MaxEntries {
		return &Error{http.StatusInsufficientStorage}
	}
	return nil
}

func (c *Cache) indexFiles() error {
	entries, err := os.ReadDir(c.cfg.Directory)
	if err != nil {
		return err
	}
	files := make(map[string]*cacheFile)
	var oldest cacheHeap
	var used int64
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".fill-") && strings.HasSuffix(entry.Name(), ".part") {
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > 2*c.cfg.Timeout {
				_ = os.Remove(filepath.Join(c.cfg.Directory, entry.Name()))
			}
		}
		if !strings.HasSuffix(entry.Name(), ".cache") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		file := &cacheFile{path: filepath.Join(c.cfg.Directory, entry.Name()), size: info.Size(), modified: info.ModTime(), index: len(oldest)}
		files[file.path] = file
		oldest = append(oldest, file)
		used += info.Size()
	}
	heap.Init(&oldest)
	c.files, c.oldest, c.used, c.indexed = files, oldest, used, true
	return nil
}

func (e *Entry) Open() (*os.File, *io.SectionReader, error) {
	f, err := os.Open(e.Path)
	if err != nil {
		return nil, nil, err
	}
	current, err := readEntryFile(f)
	if err != nil || current.ETag != e.ETag || current.Size != e.Size || !current.Saved.Equal(e.Saved) {
		_ = f.Close()
		return nil, nil, errors.New("package mirror entry changed before opening")
	}
	return f, io.NewSectionReader(f, headerSize, e.Size), nil
}
