// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

func TestServerAuthorizesDashboardAndAuditsPreviewClear(t *testing.T) {
	log := &audit.Log{}
	service := dashboard.NewService(3, []dashboard.Event{{ID: "event"}}, log)
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "dashboard:read", "events:write", "storage:read", "storage:write")), service, storage.NewService(100, []storage.Usage{{Category: storage.CategoryGitObjects, Bytes: 87}}), runner.NewService("1.0.0"))

	get := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	get.Header.Set("Authorization", "Bearer token")
	getResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want %d", getResponse.Code, http.StatusOK)
	}

	clear := httptest.NewRequest(http.MethodDelete, "/api/v1/events/preview", nil)
	clear.Header.Set("Authorization", "Bearer token")
	clearResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearResponse, clear)
	if clearResponse.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d, want %d", clearResponse.Code, http.StatusNoContent)
	}
	if entries := log.Entries(); len(entries) != 1 {
		t.Fatalf("audit entries = %#v, want one", entries)
	}
}

func TestServerReportsStorageAndProtectsProjectedWrites(t *testing.T) {
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "storage:read", "storage:write")), dashboard.NewService(1, nil, &audit.Log{}), storage.NewService(100, []storage.Usage{{Category: storage.CategoryLFSAssets, Bytes: 87}}), runner.NewService("1.0.0"))

	get := httptest.NewRequest(http.MethodGet, "/api/v1/storage", nil)
	get.Header.Set("Authorization", "Bearer token")
	getResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("storage status = %d, want %d", getResponse.Code, http.StatusOK)
	}

	admission := httptest.NewRequest(http.MethodPost, "/api/v1/storage/admission", strings.NewReader(`{"bytes":8}`))
	admission.Header.Set("Authorization", "Bearer token")
	admission.Header.Set("Content-Type", "application/json")
	admissionResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(admissionResponse, admission)
	if admissionResponse.Code != http.StatusOK || !strings.Contains(admissionResponse.Body.String(), `"write_protected":true`) {
		t.Fatalf("admission response = %d %s, want protected response", admissionResponse.Code, admissionResponse.Body.String())
	}
}

func TestServerReconcilesBundledRunner(t *testing.T) {
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "runners:write")), dashboard.NewService(1, nil, &audit.Log{}), storage.NewService(100, nil), runner.NewService("1.0.0"))

	reconcile := httptest.NewRequest(http.MethodPost, "/api/v1/runners/reconcile", strings.NewReader(`{"id":"agent-1","platform":"windows/amd64"}`))
	reconcile.Header.Set("Authorization", "Bearer token")
	reconcile.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, reconcile)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"action":"install"`) {
		t.Fatalf("reconcile response = %d %s, want install action", response.Code, response.Body.String())
	}
}
