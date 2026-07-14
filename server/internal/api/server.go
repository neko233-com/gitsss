// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/gittransport"
	"github.com/neko233-com/gitsss/server/internal/lfs"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/pipeline"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/storage"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

type Server struct {
	verifier      auth.Verifier
	dashboard     *dashboard.Service
	notifications *notification.Service
	pipelines     *pipeline.Service
	repositories  *repository.Service
	runners       *runner.Service
	storage       *storage.Service
	takeovers     *takeover.Registry
	jobs          *agentjob.Queue
	audit         *audit.Log
	git           *gittransport.Handler
	lfs           *lfs.Handler
	now           func() time.Time
}

type Dependencies struct {
	Dashboard     *dashboard.Service
	Notifications *notification.Service
	Pipelines     *pipeline.Service
	Repositories  *repository.Service
	Runners       *runner.Service
	Storage       *storage.Service
	Takeovers     *takeover.Registry
	Jobs          *agentjob.Queue
	Audit         *audit.Log
	LFS           *lfs.Handler
}

func NewServer(verifier auth.Verifier, dependencies Dependencies) *Server {
	server := &Server{
		verifier:      verifier,
		dashboard:     dependencies.Dashboard,
		notifications: dependencies.Notifications,
		pipelines:     dependencies.Pipelines,
		repositories:  dependencies.Repositories,
		runners:       dependencies.Runners,
		storage:       dependencies.Storage,
		takeovers:     dependencies.Takeovers,
		jobs:          dependencies.Jobs,
		audit:         dependencies.Audit,
		lfs:           dependencies.LFS,
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
	mux.HandleFunc("GET /api/v1/git-clients", s.gitClientCatalog)
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
	mux.HandleFunc("POST /api/v1/agent/jobs/{job}/heartbeat", s.agentJobHeartbeat)
	mux.HandleFunc("POST /api/v1/agent/jobs/{job}/complete", s.agentJobComplete)
	mux.HandleFunc("DELETE /api/v1/events/preview", s.clearPreview)
	mux.HandleFunc("GET /api/v1/storage", s.storageSnapshot)
	mux.HandleFunc("POST /api/v1/storage/admission", s.storageAdmission)
	mux.HandleFunc("GET /api/v1/runners", s.runnerList)
	mux.HandleFunc("POST /api/v1/runners/reconcile", s.runnerReconcile)
	mux.HandleFunc("GET /api/v1/pipelines/templates", s.pipelineTemplates)
	mux.HandleFunc("GET /api/v1/pipelines/runs", s.pipelineRuns)
	mux.HandleFunc("POST /api/v1/pipelines/runs", s.pipelineCreate)
	mux.HandleFunc("POST /api/v1/pipelines/runs/claim", s.pipelineClaim)
	mux.HandleFunc("POST /api/v1/pipelines/runs/{run}/heartbeat", s.pipelineHeartbeat)
	mux.HandleFunc("POST /api/v1/pipelines/runs/{run}/complete", s.pipelineComplete)
	mux.HandleFunc("GET /api/v1/repositories", s.repositoryList)
	mux.HandleFunc("POST /api/v1/repositories", s.repositoryCreate)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/tree", s.repositoryTree)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/files", s.repositoryFile)
	mux.HandleFunc("GET /api/v1/repositories/{repository}/search", s.repositorySearch)
	if s.git != nil {
		mux.Handle("/git/", http.HandlerFunc(s.gitProtocol))
	}
	return mux
}

type gitClientSource struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	URL          string `json:"url"`
	Architecture string `json:"architecture,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	Bytes        int64  `json:"bytes,omitempty"`
	Command      string `json:"command,omitempty"`
}

type gitClientPlatform struct {
	ID      string            `json:"id"`
	Label   string            `json:"label"`
	Version string            `json:"version"`
	Sources []gitClientSource `json:"sources"`
}

func (s *Server) gitClientCatalog(w http.ResponseWriter, _ *http.Request) {
	const (
		x64Digest   = "74300da8dfe0d844c5449ffb809662f8eeac47916f83730c879c4084890c6c0e"
		arm64Digest = "3df091fc297001ea9592554ee630111ea27b2d33b137859d08c4971abb319a7c"
	)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     1,
		"verified_at": "2026-07-15T00:00:00Z",
		"trust":       "embedded-release-catalog",
		"platforms": []gitClientPlatform{
			{ID: "windows", Label: "Windows 11 / 10", Version: "2.55.0.windows.2", Sources: []gitClientSource{
				{Kind: "official", Label: "官方 x64 安装包", Architecture: "x64", URL: "https://github.com/git-for-windows/git/releases/download/v2.55.0.windows.2/Git-2.55.0.2-64-bit.exe", SHA256: x64Digest, Bytes: 65449488},
				{Kind: "mirror", Label: "国内镜像 x64", Architecture: "x64", URL: "https://registry.npmmirror.com/-/binary/git-for-windows/v2.55.0.windows.2/Git-2.55.0.2-64-bit.exe", SHA256: x64Digest, Bytes: 65449488},
				{Kind: "official", Label: "官方 ARM64 安装包", Architecture: "arm64", URL: "https://github.com/git-for-windows/git/releases/download/v2.55.0.windows.2/Git-2.55.0.2-arm64.exe", SHA256: arm64Digest, Bytes: 63236912},
				{Kind: "mirror", Label: "国内镜像 ARM64", Architecture: "arm64", URL: "https://registry.npmmirror.com/-/binary/git-for-windows/v2.55.0.windows.2/Git-2.55.0.2-arm64.exe", SHA256: arm64Digest, Bytes: 63236912},
			}},
			{ID: "macos", Label: "macOS", Version: "2.55.0", Sources: []gitClientSource{
				{Kind: "official", Label: "Xcode 命令行工具", URL: "https://git-scm.com/install/mac", Architecture: "universal", Command: "xcode-select --install"},
				{Kind: "official", Label: "Homebrew", URL: "https://brew.sh/", Architecture: "universal", Command: "brew install git"},
				{Kind: "mirror", Label: "清华 Homebrew 镜像配置", URL: "https://mirrors.tuna.tsinghua.edu.cn/help/homebrew/", Architecture: "universal", Command: "brew install git"},
			}},
		},
	})
}

func (s *Server) gitProtocol(w http.ResponseWriter, r *http.Request) {
	if s.lfs != nil && strings.Contains(r.URL.Path, ".git/info/lfs/") {
		s.lfs.ServeHTTP(w, r)
		return
	}
	s.git.ServeHTTP(w, r)
}

func (s *Server) authTokens(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "auth:read")
	if !ok {
		return
	}
	tokens := s.verifier.Tokens()
	type tokenView struct {
		auth.TokenInfo
		Current bool `json:"current"`
	}
	views := make([]tokenView, 0, len(tokens))
	for _, token := range tokens {
		views = append(views, tokenView{TokenInfo: token, Current: token.ID == principal.TokenID})
	}
	writeJSON(w, http.StatusOK, views)
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
	s.touchDashboard("agent-job:"+job.ID, "已下发 Agent 任务 "+string(job.Kind), principal.TokenID, "change")
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) agentJobClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "agents:work")
	if !ok {
		return
	}
	var request struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Agent identity"})
		return
	}
	if request.AgentID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Agent identity does not match its Token"})
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

func (s *Server) agentJobHeartbeat(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "agents:work")
	if !ok {
		return
	}
	var request struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Agent heartbeat"})
		return
	}
	if request.AgentID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Agent identity does not match its Token"})
		return
	}
	job, err := s.jobs.Heartbeat(request.AgentID, r.PathValue("job"), s.now().UTC())
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

func (s *Server) agentJobComplete(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "agents:work")
	if !ok {
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
	if request.AgentID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Agent identity does not match its Token"})
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
	s.touchDashboard("takeover:"+stored.ID, "已接收接管评估 "+stored.ID, principal.TokenID, "change")
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
	s.touchDashboard("notification:"+string(state.Channel), "已更新通知通道 "+string(state.Channel), principal.TokenID, "change")
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
	snapshot := s.dashboard.SnapshotFor(principal.TokenID)
	if s.repositories != nil && s.pipelines != nil {
		runs := s.pipelines.Runs()
		active, failed := 0, 0
		for _, run := range runs {
			switch run.Status {
			case pipeline.StatusQueued, pipeline.StatusRunning:
				active++
			case pipeline.StatusFailed:
				failed++
			}
		}
		snapshot.Metrics = []dashboard.Metric{
			{Label: "仓库", Value: len(s.repositories.Repositories())},
			{Label: "运行中", Value: active},
			{Label: "构建失败", Value: failed},
		}
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) touchDashboard(id, action, actor, kind string) {
	if s.dashboard == nil {
		return
	}
	s.dashboard.Touch(dashboard.Event{
		ID: id, Action: action, Actor: actor,
		At: s.now().UTC().Format(time.RFC3339), Kind: kind,
	})
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
	writeJSON(w, http.StatusOK, s.runners.RunnersAt(s.now().UTC()))
}

func (s *Server) runnerReconcile(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "runners:write")
	if !ok {
		return
	}

	var report runner.Report
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid runner report"})
		return
	}
	reconciliation, err := s.runners.ReconcileAs(principal.TokenID, requestID(), report, s.now().UTC())
	if err != nil {
		if errors.Is(err, runner.ErrState) || errors.Is(err, audit.ErrState) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "runner state could not be updated"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, reconciliation)
}

func (s *Server) pipelineTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "pipelines:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, pipeline.Templates())
}

func (s *Server) pipelineRuns(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(w, r, "pipelines:read"); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.pipelines.Runs())
}

func (s *Server) pipelineCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "pipelines:write")
	if !ok {
		return
	}
	var request struct {
		RepositoryID string `json:"repository_id"`
		Ref          string `json:"ref"`
		TemplateID   string `json:"template_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pipeline request"})
		return
	}
	commit, err := s.repositories.Commit(r.Context(), request.RepositoryID, request.Ref)
	if err != nil {
		s.writeRepositoryError(w, err)
		return
	}
	run, err := s.pipelines.Create(principal.TokenID, requestID(), request.RepositoryID, commit, request.TemplateID, s.now().UTC())
	if err != nil {
		s.writePipelineError(w, err)
		return
	}
	s.touchDashboard("pipeline:"+run.ID, "已开始 "+run.RepositoryID+" 的 "+run.TemplateID, principal.TokenID, "build")
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) pipelineClaim(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "pipelines:work")
	if !ok {
		return
	}
	var request struct {
		RunnerID string `json:"runner_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pipeline runner"})
		return
	}
	if request.RunnerID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "runner identity does not match its Token"})
		return
	}
	claim, found, err := s.pipelines.Claim(principal.TokenID, requestID(), request.RunnerID, s.now().UTC())
	if err != nil {
		s.writePipelineError(w, err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, claim)
}

func (s *Server) pipelineHeartbeat(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "pipelines:work")
	if !ok {
		return
	}
	var request struct {
		RunnerID string `json:"runner_id"`
		Log      string `json:"log"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 40<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pipeline heartbeat"})
		return
	}
	if request.RunnerID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "runner identity does not match its Token"})
		return
	}
	run, err := s.pipelines.Heartbeat(r.PathValue("run"), request.RunnerID, request.Log, s.now().UTC())
	if err != nil {
		s.writePipelineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) pipelineComplete(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticate(w, r, "pipelines:work")
	if !ok {
		return
	}
	var request struct {
		RunnerID  string `json:"runner_id"`
		Succeeded bool   `json:"succeeded"`
		Error     string `json:"error"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pipeline completion"})
		return
	}
	if request.RunnerID != principal.TokenID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "runner identity does not match its Token"})
		return
	}
	run, err := s.pipelines.Complete(principal.TokenID, requestID(), r.PathValue("run"), request.RunnerID, request.Succeeded, request.Error, s.now().UTC())
	if err != nil {
		s.writePipelineError(w, err)
		return
	}
	if err := s.enqueuePipelineNotifications(run); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pipeline notification could not be queued"})
		return
	}
	s.touchDashboard("pipeline:"+run.ID, "流水线 "+run.RepositoryID+"："+string(run.Status), principal.TokenID, "build")
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) enqueuePipelineNotifications(run pipeline.Run) error {
	if s.notifications == nil {
		return nil
	}
	subject := "流水线执行成功"
	if run.Status == pipeline.StatusFailed {
		subject = "流水线执行失败"
	}
	commit := run.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	body := fmt.Sprintf("仓库：%s\n提交：%s\n运行：%s\n状态：%s", run.RepositoryID, commit, run.ID, run.Status)
	if run.Error != "" {
		body += "\n原因：" + run.Error
	}
	for _, channel := range s.notifications.Channels() {
		if !channel.Configured {
			continue
		}
		if _, err := s.notifications.Enqueue(channel.Channel, "pipeline:"+run.ID+":"+string(run.Status), subject, body); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) writePipelineError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	switch {
	case errors.Is(err, pipeline.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, pipeline.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, pipeline.ErrState), errors.Is(err, audit.ErrState):
		status = http.StatusInternalServerError
		message = "pipeline state could not be updated"
	}
	writeJSON(w, status, map[string]string{"error": message})
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
	s.touchDashboard("repository:"+created.ID, "已创建仓库 "+created.Name, principal.TokenID, "change")
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
