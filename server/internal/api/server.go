// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

type Server struct {
	verifier     auth.Verifier
	dashboard    *dashboard.Service
	repositories *repository.Service
	runners      *runner.Service
	storage      *storage.Service
	now          func() time.Time
}

type Dependencies struct {
	Dashboard    *dashboard.Service
	Repositories *repository.Service
	Runners      *runner.Service
	Storage      *storage.Service
}

func NewServer(verifier auth.Verifier, dependencies Dependencies) *Server {
	return &Server{
		verifier:     verifier,
		dashboard:    dependencies.Dashboard,
		repositories: dependencies.Repositories,
		runners:      dependencies.Runners,
		storage:      dependencies.Storage,
		now:          time.Now,
	}
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
	mux.HandleFunc("GET /api/v1/repositories", s.repositoryList)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/tree", s.repositoryTree)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/files", s.repositoryFile)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/search", s.repositorySearch)
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

func (s *Server) repositoryList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "repositories:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.repositories.Repositories())
}

func (s *Server) repositoryTree(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "repositories:read"); !ok {
		return
	}
	tree, err := s.repositories.Tree(r.Context(), r.PathValue("repository"), r.URL.Query().Get("ref"))
	if err != nil {
		s.writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tree)
}

func (s *Server) repositoryFile(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "repositories:read"); !ok {
		return
	}
	file, err := s.repositories.File(r.Context(), r.PathValue("repository"), r.URL.Query().Get("ref"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) repositorySearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "repositories:search"); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	matches, err := s.repositories.Search(r.Context(), r.PathValue("repository"), r.URL.Query().Get("ref"), r.URL.Query().Get("q"), limit)
	if err != nil {
		s.writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, matches)
}

func (s *Server) writeRepositoryError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "repository operation failed"
	switch {
	case errors.Is(err, repository.ErrNotFound):
		status = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, repository.ErrInvalidPath), errors.Is(err, repository.ErrInvalidQuery), errors.Is(err, repository.ErrInvalidRef):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, repository.ErrResultTooLarge):
		status = http.StatusRequestEntityTooLarge
		message = err.Error()
	}
	writeJSON(w, status, map[string]string{"error": message})
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
