// Copyright 2026 gitsss Authors. All rights reserved.

package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesReactAndKeepsAPIBoundary(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<main>gitsss</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app-123.js"), []byte("export{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	api := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-API", "yes")
		response.WriteHeader(http.StatusTeapot)
	})
	application, err := New(api, root)
	if err != nil {
		t.Fatal(err)
	}

	for _, requestPath := range []string{"/", "/repositories/team/project"} {
		request := httptest.NewRequest(http.MethodGet, requestPath, nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "gitsss") {
			t.Fatalf("%s = %d %q", requestPath, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Security-Policy") == "" || response.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s security headers = %#v", requestPath, response.Header())
		}
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/assets/app-123.js", nil)
	assetResponse := httptest.NewRecorder()
	application.ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK || assetResponse.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset = %d %#v", assetResponse.Code, assetResponse.Header())
	}

	missingRequest := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	missingResponse := httptest.NewRecorder()
	application.ServeHTTP(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing asset = %d", missingResponse.Code)
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	apiResponse := httptest.NewRecorder()
	application.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusTeapot || apiResponse.Header().Get("X-API") != "yes" {
		t.Fatalf("api = %d %#v", apiResponse.Code, apiResponse.Header())
	}
}

func TestDiscoverRootValidatesExplicitBundle(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("gitsss"), 0o600); err != nil {
		t.Fatal(err)
	}
	discovered, err := DiscoverRoot(root)
	if err != nil || discovered != filepath.Clean(root) {
		t.Fatalf("discovered = %q, %v", discovered, err)
	}
	if _, err := DiscoverRoot(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing explicit web root was accepted")
	}
}
