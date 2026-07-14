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
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

type Server struct {
	verifier  auth.Verifier
	dashboard *dashboard.Service
	runners   *runner.Service
	storage   *storage.Service
	now       func() time.Time
}

func NewServer(verifier auth.Verifier, dashboardService *dashboard.Service, storageService *storage.Service, runnerService *runner.Service) *Server {
	return &Server{verifier: verifier, dashboard: dashboardService, storage: storageService, runners: runnerService, now: time.Now}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboardSnapshot)
	mux.HandleFunc("DELETE /api/v1/events/preview", s.clearPreview)
	mux.HandleFunc("GET /api/v1/storage", s.storageSnapshot)
	mux.HandleFunc("POST /api/v1/storage/admission", s.storageAdmission)
	mux.HandleFunc("GET /api/v1/runners", s.runnerList)
	mux.HandleFunc("POST /api/v1/runners/reconcile", s.runnerReconcile)
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

func (s *Server) storageSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "storage:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.storage.Snapshot())
}

func (s *Server) storageAdmission(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "storage:write"); !ok {
		return
	}

	var request struct {
		Bytes int64 `json:"bytes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid admission request"})
		return
	}
	writeJSON(w, http.StatusOK, s.storage.Admit(request.Bytes))
}

func (s *Server) runnerList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "runners:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.runners.Runners())
}

func (s *Server) runnerReconcile(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "runners:write"); !ok {
		return
	}

	var report runner.Report
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid runner report"})
		return
	}
	reconciliation, err := s.runners.Reconcile(report, s.now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reconciliation)
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
