// Copyright 2026 gitsss Authors. All rights reserved.

package agenthttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/backup"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

func TestValidateLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8790", "[::1]:8790"} {
		if err := ValidateLoopbackAddress(address); err != nil {
			t.Fatalf("%s: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:8790", "192.168.1.2:8790", "localhost:8790", ":8790", "invalid"} {
		if err := ValidateLoopbackAddress(address); err == nil {
			t.Fatalf("%s was accepted", address)
		}
	}
}

func TestHandlerEnforcesTokensScopesAndRejectsURLTokens(t *testing.T) {
	now := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	log := &audit.Log{}
	server := NewServer(auth.NewVerifier(
		auth.NewRecord("full", "full-secret-token", now.Add(time.Hour), "agent:read", "audit:read"),
		auth.NewRecord("limited", "limited-secret-token", now.Add(time.Hour), "audit:read"),
		auth.NewRecord("expired", "expired-secret-token", now, "agent:read"),
	), log)
	server.now = func() time.Time { return now }
	handler := server.Handler()

	assertHTTPStatus(t, handler, http.MethodGet, "/healthz", "", nil, http.StatusOK)
	assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities", "", nil, http.StatusUnauthorized)
	assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities", "Bearer limited-secret-token", nil, http.StatusForbidden)
	assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities", "Bearer expired-secret-token", nil, http.StatusUnauthorized)
	assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities", "Bearerfull-secret-token", nil, http.StatusUnauthorized)
	assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities?access_token=full-secret-token", "", nil, http.StatusBadRequest)
	response := assertHTTPStatus(t, handler, http.MethodGet, "/api/v1/capabilities", "Bearer full-secret-token", nil, http.StatusOK)
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("request ID header is missing")
	}
	encoded, err := json.Marshal(log.Entries())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("full-secret-token")) || bytes.Contains(encoded, []byte("limited-secret-token")) {
		t.Fatal("audit log contains a raw token")
	}
}

func TestHandlerRunsReadOnlyAssessmentAndRecordsAudit(t *testing.T) {
	now := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	log := &audit.Log{}
	server := NewServer(auth.NewVerifier(auth.NewRecord(
		"ai-agent", "ai-agent-secret-token-2026", now.Add(time.Hour), "takeover:assess", "audit:read",
	)), log)
	server.now = func() time.Time { return now }
	configPath := filepath.Join(t.TempDir(), "app.ini")
	if err := os.WriteFile(configPath, []byte("[database]\nDB_TYPE = sqlite3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"agent_id": "ai-agent", "config_path": configPath, "max_entries": 100})
	if err != nil {
		t.Fatal(err)
	}
	response := assertHTTPStatus(t, server.Handler(), http.MethodPost, "/api/v1/assessments", "Bearer ai-agent-secret-token-2026", payload, http.StatusOK)
	var report struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&report); err != nil || !strings.HasPrefix(report.ID, "assessment-") {
		t.Fatalf("assessment response = %#v, %v", report, err)
	}
	auditResponse := assertHTTPStatus(t, server.Handler(), http.MethodGet, "/api/v1/audit", "Bearer ai-agent-secret-token-2026", nil, http.StatusOK)
	var entries []audit.Entry
	if err := json.NewDecoder(auditResponse.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	results := map[string]string{}
	for _, entry := range entries {
		results[entry.Action] = entry.Result
	}
	if len(entries) < 2 || results["audit.read"] != "success" || results["takeover.assess"] != "success" {
		t.Fatalf("unexpected audit entries: %#v", entries)
	}
}

func TestHandlerVerifiesBackupUsingEnvironmentPassphrase(t *testing.T) {
	now := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	server := NewServer(auth.NewVerifier(auth.NewRecord(
		"ai-agent", "ai-agent-backup-token-2026", now.Add(time.Hour), "backup:verify",
	)), &audit.Log{})
	server.now = func() time.Time { return now }
	directory := t.TempDir()
	configPath := filepath.Join(directory, "app.ini")
	if err := os.WriteFile(configPath, []byte("[server]\nROOT_URL = https://gitsss.local/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := backup.BuildPlan(takeover.Report{
		ID: "assessment-0123456789abcdef", ConfigPath: configPath,
		ConfigDigest: "sha256:" + strings.Repeat("a", 64),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(directory, "test.gitsss-backup")
	const passphrase = "agent HTTP backup passphrase"
	if err := backup.Export(backupPath, passphrase, plan); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITSSS_AGENT_HTTP_TEST_PASSPHRASE", passphrase)
	payload, err := json.Marshal(map[string]string{
		"input_path": backupPath, "passphrase_env": "GITSSS_AGENT_HTTP_TEST_PASSPHRASE",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := assertHTTPStatus(t, server.Handler(), http.MethodPost, "/api/v1/backups/verify", "Bearer ai-agent-backup-token-2026", payload, http.StatusOK)
	var verification backup.Verification
	if err := json.NewDecoder(response.Body).Decode(&verification); err != nil || verification.Files != 1 {
		t.Fatalf("verification = %#v, %v", verification, err)
	}
}

func assertHTTPStatus(t *testing.T, handler http.Handler, method, target, authorization string, body []byte, expected int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expected {
		t.Fatalf("%s %s status = %d, want %d: %s", method, target, response.Code, expected, response.Body.String())
	}
	return response
}
