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
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/gittransport"
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
	audit         *audit.Log
	git           *gittransport.Handler
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
	Audit         *audit.Log
}

func NewServer(verifier auth.Verifier, dependencies Dependencies) *Server {
	server := &Server{
		verifier:      verifier,
		dashboard:     dependencies.Dashboard,
		notifications: dependencies.Notifications,
		repositories:  dependencies.Repositories,
		runners:       dependencies.Runners,
		storage:       dependencies.Storage,
		takeovers:     dependencies.Takeovers,
		jobs:          dependencies.Jobs,
		audit:         dependencies.Audit,
		now:           time.Now,
	}
	if dependencies.Repositories != nil && dependencies.Audit != nil {
		server.git = gittransport.NewHandler(verifier, dependencies.Repositories, dependencies.Audit)
	}
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/auth/tokens", s.authTokens)
	mux.HandleFunc("POST /api/v1/auth/tokens", s.authTokenIssue)
	mux.HandleFunc("DELETE /api/v1/auth/tokens/{token}", s.authTokenRevoke)
	mux.HandleFunc("GET /api/v1/auth/roles", s.authRoles)
	mux.HandleFunc("GET /api/v1/auth/roles/{role}/preview", s.authRolePreview)
	mux.HandleFunc("GET /api/v1/dashboard", s.dashboardSnapshot)
	mux.HandleFunc("GET /api/v1/notifications/channels", s.notificationChannels)
	mux.HandleFunc("POST /api/v1/notifications/channels/{channel}", s.notificationConfigure)
	mux.HandleFunc("POST /api/v1/notifications/channels/{channel}/test", s.notificationTest)
	mux.HandleFunc("GET /api/v1/notifications/outbox", s.notificationOutbox)
	mux.HandleFunc("POST /api/v1/notifications/dispatch", s.notificationDispatch)
	mux.HandleFunc("GET /api/v1/takeover/assessments", s.takeoverAssessments)
	mux.HandleFunc("POST /api/v1/takeover/assessments", s.takeoverAssessmentSubmit)
	mux.HandleFunc("GET /api/v1/agent/jobs", s.agentJobs)
	mux.HandleFunc("GET /api/v1/agent/health", s.agentHealth)
	mux.HandleFunc("POST /api/v1/agent/jobs", s.agentJobCreate)
	mux.HandleFunc("POST /api/v1/agent/jobs/claim", s.agentJobClaim)
	mux.HandleFunc("POST /api/v1/agent/jobs/{job}/complete", s.agentJobComplete)
	mux.HandleFunc("DELETE /api/v1/events/preview", s.clearPreview)
	mux.HandleFunc("GET /api/v1/storage", s.storageSnapshot)
	mux.HandleFunc("POST /api/v1/storage/admission", s.storageAdmission)
	mux.HandleFunc("GET /api/v1/runners", s.runnerList)
	mux.HandleFunc("POST /api/v1/runners/reconcile", s.runnerReconcile)
	mux.HandleFunc("GET /api/v1/repositories", s.repositoryList)
	mux.HandleFunc("POST /api/v1/repositories", s.repositoryCreate)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/tree", s.repositoryTree)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/files", s.repositoryFile)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/search", s.repositorySearch)
	if s.git != nil {
		mux.Handle("/git/", s.git)
	}
	return mux
}

func (s *Server) authTokens(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "auth:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.verifier.Tokens())
}

func (s *Server) authRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "auth:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, auth.Roles())
}

func (s *Server) authRolePreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "auth:read"); !ok {
		return
	}
	template, err := auth.Role(r.PathValue("role"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	hours := template.DefaultHours
	if configured := r.URL.Query().Get("hours"); configured != "" {
		hours, err = strconv.Atoi(configured)
		if err != nil || hours < 1 || hours > template.MaximumHours {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role duration exceeds the allowed window"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"role": template, "expires_at": s.now().UTC().Add(time.Duration(hours) * time.Hour),
	})
}

func (s *Server) authTokenIssue(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "auth:write")
	if !ok {
		return
	}
	var request struct {
		ID             string   `json:"id"`
		Role           string   `json:"role"`
		Scopes         []string `json:"scopes"`
		ExpiresInHours int      `json:"expires_in_hours"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid token request"})
		return
	}
	now := s.now().UTC()
	var issued auth.IssuedToken
	var err error
	if request.Role != "" {
		template, roleErr := auth.Role(request.Role)
		if roleErr != nil || len(request.Scopes) != 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "select one role or provide advanced scopes"})
			return
		}
		if request.ExpiresInHours == 0 {
			request.ExpiresInHours = template.DefaultHours
		}
		issued, err = s.verifier.IssueForRole(request.ID, request.Role, now, time.Duration(request.ExpiresInHours)*time.Hour)
	} else {
		if request.ExpiresInHours < 1 || request.ExpiresInHours > 365*24 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid token request"})
			return
		}
		issued, err = s.verifier.Issue(request.ID, request.Scopes, now, now.Add(time.Duration(request.ExpiresInHours)*time.Hour))
	}
	if err != nil {
		s.writeAuthError(w, err)
		return
	}
	if s.audit != nil {
		entry := audit.Entry{Actor: principal.TokenID, Action: "auth.token.issue", Target: issued.ID, RequestID: requestID(), At: now}
		if err := s.audit.Append(entry); err != nil {
			_ = s.verifier.Revoke(issued.ID, principal.TokenID, now)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit state could not be updated"})
			return
		}
	}
	writeJSON(w, http.StatusCreated, issued)
}

func (s *Server) authTokenRevoke(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "auth:write")
	if !ok {
		return
	}
	id := r.PathValue("token")
	now := s.now().UTC()
	if err := s.verifier.Revoke(id, principal.TokenID, now); err != nil {
		s.writeAuthError(w, err)
		return
	}
	if s.audit != nil {
		if err := s.audit.Append(audit.Entry{Actor: principal.TokenID, Action: "auth.token.revoke", Target: id, RequestID: requestID(), At: now}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit state could not be updated"})
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeAuthError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	if errors.Is(err, auth.ErrState) {
		status = http.StatusInternalServerError
		message = "authentication state could not be updated"
	} else if errors.Is(err, auth.ErrTokenNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, auth.ErrTokenExists) {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) agentHealth(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "agents:work")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token_id": principal.TokenID})
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
		if errors.Is(err, audit.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit state could not be updated"})
			return
		}
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
		if errors.Is(err, audit.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit state could not be updated"})
			return
		}
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
		if errors.Is(err, audit.ErrState) {
			status = http.StatusInternalServerError
		} else if errors.Is(err, agentjob.ErrNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, agentjob.ErrWrongAgent) {
			status = http.StatusForbidden
		}
		message := err.Error()
		if errors.Is(err, audit.ErrState) {
			message = "audit state could not be updated"
		}
		writeJSON(w, status, map[string]string{"error": message})
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
		if errors.Is(err, audit.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "audit state could not be updated"})
			return
		}
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
		if errors.Is(err, notification.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification state could not be updated"})
			return
		}
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
		if errors.Is(err, notification.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification state could not be updated"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	processed, err := s.notifications.DispatchDue(r.Context(), principal.TokenID, id, 1)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification state could not be updated"})
		return
	}
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
	processed, err := s.notifications.DispatchDue(r.Context(), principal.TokenID, requestID(), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification state could not be updated"})
		return
	}
	writeJSON(w, http.StatusOK, processed)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) dashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "dashboard:read")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.dashboard.SnapshotFor(principal.TokenID))
}

func (s *Server) clearPreview(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "events:write")
	if !ok {
		return
	}
	if err := s.dashboard.ClearPreview(principal.TokenID, requestID(), s.now().UTC()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dashboard preview could not be updated"})
		return
	}
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

func (s *Server) repositoryCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "repositories:write")
	if !ok {
		return
	}
	var input repository.CreateInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository request"})
		return
	}
	created, err := s.repositories.Create(principal.TokenID, requestID(), input, s.now().UTC())
	if err != nil {
		s.writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
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
	case errors.Is(err, repository.ErrExists):
		status = http.StatusConflict
		message = err.Error()
	case errors.Is(err, repository.ErrState), errors.Is(err, audit.ErrState):
		status = http.StatusInternalServerError
		message = "repository state could not be updated"
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, scope string) (auth.Principal, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": auth.ErrInvalidToken.Error()})
		return auth.Principal{}, false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" || strings.TrimSpace(token) != token || strings.ContainsAny(token, " \t\r\n\x00") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": auth.ErrInvalidToken.Error()})
		return auth.Principal{}, false
	}
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
