// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelHTTPRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789"), 50)
	var mu sync.Mutex
	stored := map[string][]byte{}
	var item []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/uploads"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"up","size":`+strconv.Itoa(len(payload))+`,"received":0}`)
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/uploads/up"):
			span, err := ParseContentRange(r.Header.Get("Content-Range"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			buf := make([]byte, span.Length())
			if _, err := io.ReadFull(r.Body, buf); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			slot := stored["up"]
			if slot == nil {
				slot = make([]byte, len(payload))
			}
			copy(slot[span.Start:span.End+1], buf)
			stored["up"] = slot
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"up","received":1,"size":1}`)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/complete"):
			mu.Lock()
			item = append([]byte(nil), stored["up"]...)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":7,"name":"kit.bin","size":`+strconv.Itoa(len(payload))+`,"link":"/files/7"}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/files/7"):
			http.ServeContent(w, r, "kit.bin", time.Unix(0, 0), bytes.NewReader(item))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	source := filepath.Join(dir, "kit.bin")
	require.NoError(t, os.WriteFile(source, payload, 0o600))
	client := HTTPClient{Client: server.Client(), BaseURL: server.URL + "/shares", Parallel: 4, ChunkSize: 64}
	created, err := client.PutFile(t.Context(), source, "kit.bin", false)
	require.NoError(t, err)
	assert.Equal(t, int64(7), created.ID)
	assert.Equal(t, payload, item)

	dest := filepath.Join(dir, "out.bin")
	require.NoError(t, client.GetFile(t.Context(), server.URL+"/files/7", dest))
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}
