// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
)

type Server struct {
	verifier  auth.Verifier
	dashboard *dashboard.Service
	now       func() time.Time
}

func NewServer(verifier auth.Verifier, dashboardService *dashboard.Service) *Server {
	return &Server{verifier: verifier, dashboard: dashboardService, now: time.Now}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboardSnapshot)
	mux.HandleFunc("DELETE /api/v1/events/preview", s.clearPreview)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) dashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "dashboard:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.dashboard.Snapshot())
}

func (s *Server) clearPreview(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "events:write")
	if !ok {
		return
	}
	s.dashboard.ClearPreview(principal.TokenID, requestID(), s.now().UTC())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, scope string) (auth.Principal, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	principal, err := s.verifier.Verify(token, scope, s.now())
	if err == nil {
		return principal, true
	}

	status := http.StatusUnauthorized
	if errors.Is(err, auth.ErrMissingScope) {
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
	return auth.Principal{}, false
}

func requestID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(bytes)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
