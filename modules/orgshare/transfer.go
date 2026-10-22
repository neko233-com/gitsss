// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
	"golang.org/x/sync/errgroup"
)

// HTTPClient uploads and downloads share objects with parallel byte ranges.
type HTTPClient struct {
	Client    *http.Client
	BaseURL   string
	Token     string
	Parallel  int
	ChunkSize int64
}

// HTTPItem is one object created by PutFile.
type HTTPItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Link string `json:"link"`
}

type uploadSession struct {
	ID       string `json:"id"`
	Size     int64  `json:"size"`
	Received int64  `json:"received"`
}

// PutFile splits a local file into chunks and uploads them concurrently.
func (c HTTPClient) PutFile(ctx context.Context, filePath, name string, permanent bool) (HTTPItem, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return HTTPItem{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return HTTPItem{}, err
	}
	if name == "" {
		name = info.Name()
	}
	body, err := json.Marshal(map[string]any{"name": name, "size": info.Size(), "permanent": permanent, "resume": true})
	if err != nil {
		return HTTPItem{}, err
	}
	response, err := c.do(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/uploads", strings.NewReader(string(body)), int64(len(body)), "")
	if err != nil {
		return HTTPItem{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return HTTPItem{}, httpError(response)
	}
	var session uploadSession
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		return HTTPItem{}, err
	}
	if err := c.putChunks(ctx, file, session.ID, info.Size()); err != nil {
		return HTTPItem{}, err
	}
	done, err := c.do(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/uploads/"+session.ID+"/complete", nil, 0, "")
	if err != nil {
		return HTTPItem{}, err
	}
	defer done.Body.Close()
	if done.StatusCode != http.StatusCreated && done.StatusCode != http.StatusOK {
		return HTTPItem{}, httpError(done)
	}
	var item HTTPItem
	if err := json.NewDecoder(done.Body).Decode(&item); err != nil {
		return HTTPItem{}, err
	}
	return item, nil
}

func (c HTTPClient) putChunks(ctx context.Context, file *os.File, uploadID string, size int64) error {
	spans := PlanChunks(size, NormalizeChunk(c.ChunkSize))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(NormalizeParallel(c.Parallel))
	for _, span := range spans {
		group.Go(func() error {
			chunk := io.NewSectionReader(file, span.Start, span.End-span.Start)
			header := fmt.Sprintf("bytes %d-%d/%d", span.Start, span.End-1, size)
			response, err := c.do(groupCtx, http.MethodPatch, strings.TrimRight(c.BaseURL, "/")+"/uploads/"+uploadID, chunk, span.End-span.Start, header)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
				return httpError(response)
			}
			return nil
		})
	}
	return group.Wait()
}

// GetFile downloads url with parallel Range requests into dest.
func (c HTTPClient) GetFile(ctx context.Context, fileURL, dest string) error {
	size, err := c.contentLength(ctx, fileURL)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Truncate(size); err != nil {
		return err
	}
	spans := PlanChunks(size, NormalizeChunk(c.ChunkSize))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(NormalizeParallel(c.Parallel))
	for _, span := range spans {
		group.Go(func() error {
			request, err := http.NewRequestWithContext(groupCtx, http.MethodGet, fileURL, nil)
			if err != nil {
				return err
			}
			c.authorize(request)
			request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", span.Start, span.End-1))
			response, err := c.client().Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusPartialContent && !(span.Start == 0 && span.End == size && response.StatusCode == http.StatusOK) {
				return httpError(response)
			}
			buf := make([]byte, 1<<20)
			_, err = io.CopyBuffer(io.NewOffsetWriter(file, span.Start), io.LimitReader(response.Body, span.End-span.Start), buf)
			return err
		})
	}
	return group.Wait()
}

func (c HTTPClient) contentLength(ctx context.Context, fileURL string) (int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return 0, err
	}
	c.authorize(request)
	request.Header.Set("Range", "bytes=0-0")
	response, err := c.client().Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode == http.StatusPartialContent {
		return parseTotal(response.Header.Get("Content-Range"))
	}
	if response.StatusCode == http.StatusOK && response.ContentLength >= 0 {
		return response.ContentLength, nil
	}
	return 0, httpError(response)
}

func (c HTTPClient) do(ctx context.Context, method, url string, body io.Reader, length int64, contentRange string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	c.authorize(request)
	if length > 0 {
		request.ContentLength = length
	}
	if contentRange != "" {
		request.Header.Set("Content-Range", contentRange)
		request.Header.Set("Content-Type", "application/octet-stream")
	} else if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return c.client().Do(request)
}

func (c HTTPClient) authorize(request *http.Request) {
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

func (c HTTPClient) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func parseTotal(header string) (int64, error) {
	_, total, ok := strings.Cut(header, "/")
	if !ok {
		return 0, fmt.Errorf("content range %q has no total", header)
	}
	return strconv.ParseInt(strings.TrimSpace(total), 10, 64)
}

func httpError(response *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	text := strings.TrimSpace(string(payload))
	if text == "" {
		text = response.Status
	}
	return fmt.Errorf("share request failed: %s", text)
}
