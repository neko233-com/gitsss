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

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

type Server struct {
	verifier      auth.Verifier
	dashboard     *dashboard.Service
	notifications *notification.Service
	repositories  *repository.Service
	runners       *runner.Service
	storage       *storage.Service
	takeovers     *takeover.Registry
	jobs          *agentjob.Queue
	now           func() time.Time
}

type Dependencies struct {
	Dashboard     *dashboard.Service
	Notifications *notification.Service
	Repositories  *repository.Service
	Runners       *runner.Service
	Storage       *storage.Service
	Takeovers     *takeover.Registry
	Jobs          *agentjob.Queue
}

func NewServer(verifier auth.Verifier, dependencies Dependencies) *Server {
	return &Server{
		verifier:      verifier,
		dashboard:     dependencies.Dashboard,
		notifications: dependencies.Notifications,
		repositories:  dependencies.Repositories,
		runners:       dependencies.Runners,
		storage:       dependencies.Storage,
		takeovers:     dependencies.Takeovers,
		jobs:          dependencies.Jobs,
		now:           time.Now,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboardSnapshot)
	mux.HandleFunc("GET /api/v1/notifications/channels", s.notificationChannels)
	mux.HandleFunc("POST /api/v1/notifications/channels/{channel}", s.notificationConfigure)
	mux.HandleFunc("POST /api/v1/notifications/channels/{channel}/test", s.notificationTest)
	mux.HandleFunc("GET /api/v1/notifications/outbox", s.notificationOutbox)
	mux.HandleFunc("POST /api/v1/notifications/dispatch", s.notificationDispatch)
	mux.HandleFunc("GET /api/v1/takeover/assessments", s.takeoverAssessments)
	mux.HandleFunc("POST /api/v1/takeover/assessments", s.takeoverAssessmentSubmit)
	mux.HandleFunc("GET /api/v1/agent/jobs", s.agentJobs)
	mux.HandleFunc("POST /api/v1/agent/jobs", s.agentJobCreate)
	mux.HandleFunc("POST /api/v1/agent/jobs/claim", s.agentJobClaim)
	mux.HandleFunc("POST /api/v1/agent/jobs/{job}/complete", s.agentJobComplete)
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

func (s *Server) agentJobs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "agents:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.jobs.Jobs())
}

func (s *Server) agentJobCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "agents:dispatch")
	if !ok {
		return
	}
	var request struct {
		Kind  agentjob.Kind   `json:"kind"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 40<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Agent job"})
		return
	}
	job, err := s.jobs.Create(principal.TokenID, requestID(), request.Kind, request.Input, s.now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) agentJobClaim(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "agents:work"); !ok {
		return
	}
	var request struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Agent identity"})
		return
	}
	job, found, err := s.jobs.Claim(request.AgentID, requestID(), s.now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) agentJobComplete(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "agents:work"); !ok {
		return
	}
	var request struct {
		AgentID string          `json:"agent_id"`
		Result  json.RawMessage `json:"result"`
		Error   string          `json:"error"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 264<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Agent completion"})
		return
	}
	job, err := s.jobs.Complete(request.AgentID, requestID(), r.PathValue("job"), request.Result, request.Error, s.now().UTC())
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, agentjob.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, agentjob.ErrWrongAgent) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) takeoverAssessments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "takeover:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.takeovers.Reports())
}

func (s *Server) takeoverAssessmentSubmit(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "takeover:write")
	if !ok {
		return
	}
	var report takeover.Report
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid takeover assessment"})
		return
	}
	stored, err := s.takeovers.Submit(principal.TokenID, requestID(), report, s.now().UTC())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

func (s *Server) notificationChannels(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "notifications:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.notifications.Channels())
}

func (s *Server) notificationConfigure(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "notifications:write")
	if !ok {
		return
	}
	var configuration notification.Configuration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&configuration); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification configuration"})
		return
	}
	configuration.Channel = notification.Channel(r.PathValue("channel"))
	state, err := s.notifications.Configure(principal.TokenID, requestID(), configuration)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) notificationTest(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "notifications:write")
	if !ok {
		return
	}
	var request struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid test notification"})
		return
	}
	if strings.TrimSpace(request.Subject) == "" {
		request.Subject = "gitsss 通知测试"
	}
	if strings.TrimSpace(request.Body) == "" {
		request.Body = "连接成功。之后的重要事件会通过此通道发送。"
	}
	id := requestID()
	if _, err := s.notifications.Enqueue(notification.Channel(r.PathValue("channel")), id, request.Subject, request.Body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	processed := s.notifications.DispatchDue(r.Context(), principal.TokenID, id, 1)
	if len(processed) == 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "notification is waiting for retry"})
		return
	}
	writeJSON(w, http.StatusOK, processed[0])
}

func (s *Server) notificationOutbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "notifications:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.notifications.Messages())
}

func (s *Server) notificationDispatch(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "notifications:dispatch")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.notifications.DispatchDue(r.Context(), principal.TokenID, requestID(), 20))
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
