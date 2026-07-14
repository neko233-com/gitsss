// Copyright 2026 gitsss Authors. All rights reserved.

package agenthttp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/backup"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

const auditRetention = 30 * 24 * time.Hour

type Server struct {
	verifier auth.Verifier
	audit    *audit.Log
	now      func() time.Time
}

func NewServer(verifier auth.Verifier, auditLog *audit.Log) *Server {
	return &Server{verifier: verifier, audit: auditLog, now: time.Now}
}

func ValidateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid Agent HTTP listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("Agent HTTP must listen on a loopback address")
	}
	return nil
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/v1/capabilities", server.capabilities)
	mux.HandleFunc("POST /api/v1/assessments", server.assess)
	mux.HandleFunc("POST /api/v1/backups/verify", server.verifyBackup)
	mux.HandleFunc("GET /api/v1/audit", server.auditEntries)
	return server.guard(mux)
}

func (server *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		id := newRequestID()
		response.Header().Set("X-Request-ID", id)
		for key := range request.URL.Query() {
			if strings.Contains(strings.ToLower(key), "token") {
				server.record("anonymous", "http.reject_query_token", request.URL.Path, id, "rejected")
				writeJSON(response, http.StatusBadRequest, map[string]string{"error": "tokens are not accepted in URL query parameters"})
				return
			}
		}
		request.Header.Set("X-gitsss-Request-ID", id)
		next.ServeHTTP(response, request)
	})
}

func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "service": "gitsss-agent"})
}

func (server *Server) capabilities(response http.ResponseWriter, request *http.Request) {
	principal, ok := server.authorize(response, request, "agent:read", "agent.capabilities")
	if !ok {
		return
	}
	server.record(principal.TokenID, "agent.capabilities", "local-agent", requestID(request), "success")
	writeJSON(response, http.StatusOK, map[string]any{
		"version":      1,
		"capabilities": []string{"takeover.assess.read-only", "backup.verify.read-only", "audit.read"},
	})
}

func (server *Server) assess(response http.ResponseWriter, request *http.Request) {
	principal, ok := server.authorize(response, request, "takeover:assess", "takeover.assess")
	if !ok {
		return
	}
	var options takeover.Options
	if err := decodeJSON(response, request, 32<<10, &options); err != nil {
		server.record(principal.TokenID, "takeover.assess", "configuration", requestID(request), "invalid")
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid assessment request"})
		return
	}
	report, err := takeover.Assess(options)
	if err != nil {
		server.record(principal.TokenID, "takeover.assess", "configuration", requestID(request), "failed")
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	server.record(principal.TokenID, "takeover.assess", report.ID, requestID(request), "success")
	writeJSON(response, http.StatusOK, report)
}

func (server *Server) verifyBackup(response http.ResponseWriter, request *http.Request) {
	principal, ok := server.authorize(response, request, "backup:verify", "backup.verify")
	if !ok {
		return
	}
	var input struct {
		InputPath     string `json:"input_path"`
		PassphraseEnv string `json:"passphrase_env"`
	}
	if err := decodeJSON(response, request, 8<<10, &input); err != nil || !validEnvironmentName(input.PassphraseEnv) || strings.TrimSpace(input.InputPath) == "" {
		server.record(principal.TokenID, "backup.verify", "backup", requestID(request), "invalid")
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "input_path and a valid passphrase_env are required"})
		return
	}
	passphrase := os.Getenv(input.PassphraseEnv)
	if passphrase == "" {
		server.record(principal.TokenID, "backup.verify", "backup", requestID(request), "failed")
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "backup passphrase environment variable is empty"})
		return
	}
	verification, err := backup.Verify(input.InputPath, passphrase)
	if err != nil {
		server.record(principal.TokenID, "backup.verify", "backup", requestID(request), "failed")
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	server.record(principal.TokenID, "backup.verify", verification.Manifest.Source.AssessmentID, requestID(request), "success")
	writeJSON(response, http.StatusOK, verification)
}

func (server *Server) auditEntries(response http.ResponseWriter, request *http.Request) {
	principal, ok := server.authorize(response, request, "audit:read", "audit.read")
	if !ok {
		return
	}
	server.record(principal.TokenID, "audit.read", "local-agent", requestID(request), "success")
	entries := server.audit.Entries()
	if len(entries) > 500 {
		entries = entries[:500]
	}
	writeJSON(response, http.StatusOK, entries)
}

func (server *Server) authorize(response http.ResponseWriter, request *http.Request, scope, action string) (auth.Principal, bool) {
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		server.record("anonymous", action, request.URL.Path, requestID(request), "unauthorized")
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": auth.ErrInvalidToken.Error()})
		return auth.Principal{}, false
	}
	principal, err := server.verifier.Verify(parts[1], scope, server.now())
	if err == nil {
		return principal, true
	}
	status := http.StatusUnauthorized
	if errors.Is(err, auth.ErrMissingScope) {
		status = http.StatusForbidden
	}
	server.record("anonymous", action, request.URL.Path, requestID(request), "unauthorized")
	writeJSON(response, status, map[string]string{"error": err.Error()})
	return auth.Principal{}, false
}

func (server *Server) record(actor, action, target, requestID, result string) {
	now := server.now().UTC()
	_, _ = server.audit.Cleanup(now.Add(-auditRetention))
	_ = server.audit.Append(audit.Entry{Actor: actor, Action: action, Target: target, RequestID: requestID, Result: result, At: now})
}

func decodeJSON(response http.ResponseWriter, request *http.Request, limit int64, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func validEnvironmentName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func requestID(request *http.Request) string {
	return request.Header.Get("X-gitsss-Request-ID")
}

func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(value)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
