// Copyright 2026 gitsss Authors. All rights reserved.

package lfs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

func TestHandlerUploadsDownloadsDeduplicatesAndResumes(t *testing.T) {
	handler, log := newTestHandler(t, storage.NewService(1<<30, nil))
	server := httptest.NewServer(handler)
	defer server.Close()

	content := []byte("PSD\x00game-asset")
	oid := objectOID(content)
	batch := batchLFS(t, server.URL, "push-token", "upload", oid, int64(len(content)))
	if batch.Objects[0].Actions["upload"].Href == "" {
		t.Fatalf("upload batch = %#v", batch)
	}
	uploadLFS(t, server.URL, "push-token", oid, content, "", http.StatusOK)
	deduplicated := batchLFS(t, server.URL, "push-token", "upload", oid, int64(len(content)))
	if len(deduplicated.Objects[0].Actions) != 0 {
		t.Fatalf("deduplicated batch = %#v", deduplicated)
	}
	download := batchLFS(t, server.URL, "clone-token", "download", oid, int64(len(content)))
	if download.Objects[0].Actions["download"].Href == "" {
		t.Fatalf("download batch = %#v", download)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/git/game.git/info/lfs/objects/"+oid, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer clone-token")
	request.Header.Set("Range", "bytes=4-7")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	rangeContent, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusPartialContent || string(rangeContent) != string(content[4:8]) {
		t.Fatalf("range response = %d %q", response.StatusCode, rangeContent)
	}

	resumable := []byte("resumable")
	resumableOID := objectOID(resumable)
	uploadLFS(t, server.URL, "push-token", resumableOID, resumable[:4], "bytes 0-3/9", http.StatusAccepted)
	uploadLFS(t, server.URL, "push-token", resumableOID, resumable[4:], "bytes 4-8/9", http.StatusOK)
	if info, err := regularFileInfo(handler.objectPath(resumableOID)); err != nil || info.Size() != int64(len(resumable)) {
		t.Fatalf("resumable object = %#v, %v", info, err)
	}
	if entries := log.Entries(); len(entries) < 8 {
		t.Fatalf("LFS audit entries = %#v", entries)
	}
}

func TestHandlerRejectsWrongHashUnsafeAuthAndStoragePressure(t *testing.T) {
	protected, _ := newTestHandler(t, storage.NewService(100, []storage.Usage{{Category: storage.CategoryGitObjects, Bytes: 96}}))
	protectedServer := httptest.NewServer(protected)
	defer protectedServer.Close()
	content := []byte("asset")
	oid := objectOID(content)
	batch := batchLFS(t, protectedServer.URL, "push-token", "upload", oid, int64(len(content)))
	if batch.Objects[0].Error == nil || batch.Objects[0].Error.Code != http.StatusInsufficientStorage {
		t.Fatalf("storage-protected batch = %#v", batch)
	}

	handler, _ := newTestHandler(t, storage.NewService(1<<30, nil))
	server := httptest.NewServer(handler)
	defer server.Close()
	wrong := strings.Repeat("a", 64)
	uploadLFS(t, server.URL, "push-token", wrong, content, "", http.StatusBadRequest)
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/git/game.git/info/lfs/objects/"+oid, nil)
	request.Header.Set("Authorization", "Bearerclone-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("malformed auth status = %d", response.StatusCode)
	}
}

func newTestHandler(t *testing.T, storageService *storage.Service) (*Handler, *audit.Log) {
	t.Helper()
	repositoryPath := filepath.Join(t.TempDir(), "game.git")
	command := exec.Command("git", "init", "--bare", "--initial-branch=main", repositoryPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	repositories, err := repository.NewService([]repository.Repository{{ID: "game", Name: "Game", Path: repositoryPath}})
	if err != nil {
		t.Fatal(err)
	}
	log := &audit.Log{}
	handler, err := NewHandler(Options{Root: filepath.Join(t.TempDir(), "lfs")}, auth.NewVerifier(
		auth.NewRecord("writer", "push-token", time.Time{}, "repositories:push"),
		auth.NewRecord("reader", "clone-token", time.Time{}, "repositories:clone"),
	), repositories, storageService, log)
	if err != nil {
		t.Fatal(err)
	}
	return handler, log
}

func batchLFS(t *testing.T, serverURL, token, operation, oid string, size int64) struct {
	Objects []objectResponse `json:"objects"`
} {
	t.Helper()
	payload, _ := json.Marshal(batchRequest{Operation: operation, Transfers: []string{"basic"}, HashAlgo: "sha256", Objects: []objectRequest{{OID: oid, Size: size}}})
	request, err := http.NewRequest(http.MethodPost, serverURL+"/git/game.git/info/lfs/objects/batch", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", mediaType)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("batch status = %d: %s", response.StatusCode, body)
	}
	var result struct {
		Objects []objectResponse `json:"objects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func uploadLFS(t *testing.T, serverURL, token, oid string, content []byte, contentRange string, expectedStatus int) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, serverURL+"/git/game.git/info/lfs/objects/"+oid, bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if contentRange != "" {
		request.Header.Set("Content-Range", contentRange)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != expectedStatus {
		t.Fatalf("upload status = %d, want %d: %s", response.StatusCode, expectedStatus, body)
	}
}

func objectOID(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
