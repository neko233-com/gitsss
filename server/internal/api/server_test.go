// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
)

func TestServerAuthorizesDashboardAndAuditsPreviewClear(t *testing.T) {
	log := &audit.Log{}
	service := dashboard.NewService(3, []dashboard.Event{{ID: "event"}}, log)
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "dashboard:read", "events:write")), service)

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
