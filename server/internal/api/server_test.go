// Copyright 2026 gitsss Authors. All rights reserved.

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/agentjob"
	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/dashboard"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/runner"
	"github.com/neko233-com/gitsss/server/internal/state"
	"github.com/neko233-com/gitsss/server/internal/storage"
	"github.com/neko233-com/gitsss/server/internal/takeover"
)

type notificationFactory struct{}

func (notificationFactory) Build(notification.Configuration) (notification.Adapter, error) {
	return notificationAdapter{}, nil
}

type notificationAdapter struct{}

func (notificationAdapter) Deliver(context.Context, notification.Message) error {
	return nil
}

func TestServerManagesPersistentTokensWithoutReturningSecretsAgain(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vault, err := state.OpenVault(filepath.Join(root, "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	auditLog, err := audit.NewPersistentLog(store, vault)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := auth.NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	bootstrap := strings.Repeat("b", 24)
	if _, err := verifier.EnsureBootstrap(bootstrap, now, now.Add(24*time.Hour), "auth:read", "auth:write"); err != nil {
		t.Fatal(err)
	}
	server := NewServer(verifier, Dependencies{Audit: auditLog})
	server.now = func() time.Time { return now }
	handler := server.Handler()

	issue := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"id":"desktop-admin","scopes":["auth:read"],"expires_in_hours":48}`))
	issue.Header.Set("Authorization", "Bearer "+bootstrap)
	issueResponse := httptest.NewRecorder()
	handler.ServeHTTP(issueResponse, issue)
	if issueResponse.Code != http.StatusCreated {
		t.Fatalf("issue = %d %s", issueResponse.Code, issueResponse.Body.String())
	}
	var issued auth.IssuedToken
	if err := json.Unmarshal(issueResponse.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, "gitsss_") {
		t.Fatalf("issued token = %q", issued.Token)
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	list.Header.Set("Authorization", "Bearer "+issued.Token)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), issued.Token) || strings.Contains(listResponse.Body.String(), `"hash"`) {
		t.Fatalf("list = %d %s", listResponse.Code, listResponse.Body.String())
	}

	revoke := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/desktop-admin", nil)
	revoke.Header.Set("Authorization", "Bearer "+bootstrap)
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, list)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d", denied.Code)
	}
	entries := auditLog.Entries()
	actions := make(map[string]bool, len(entries))
	for _, entry := range entries {
		actions[entry.Action] = true
	}
	if len(entries) != 2 || !actions["auth.token.issue"] || !actions["auth.token.revoke"] {
		t.Fatalf("audit entries = %#v", entries)
	}
}

func TestServerAuthorizesDashboardAndAuditsPreviewClear(t *testing.T) {
	log := &audit.Log{}
	service := dashboard.NewService(3, []dashboard.Event{{ID: "event"}}, log)
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "dashboard:read", "events:write", "storage:read", "storage:write")), Dependencies{Dashboard: service, Runners: runner.NewService("1.0.0"), Storage: storage.NewService(100, []storage.Usage{{Category: storage.CategoryGitObjects, Bytes: 87}})})

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

func TestServerKeepsDashboardPreviewClearScopedToToken(t *testing.T) {
	log := &audit.Log{}
	service := dashboard.NewService(2, []dashboard.Event{{ID: "event-1"}}, log)
	server := NewServer(auth.NewVerifier(
		auth.NewRecord("user-a", "token-a", time.Time{}, "dashboard:read", "events:write"),
		auth.NewRecord("user-b", "token-b", time.Time{}, "dashboard:read", "events:write"),
	), Dependencies{Dashboard: service})

	clear := httptest.NewRequest(http.MethodDelete, "/api/v1/events/preview", nil)
	clear.Header.Set("Authorization", "Bearer token-a")
	clearResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearResponse, clear)
	if clearResponse.Code != http.StatusNoContent {
		t.Fatalf("clear = %d %s", clearResponse.Code, clearResponse.Body.String())
	}
	for _, check := range []struct {
		token      string
		wantEvents string
	}{
		{token: "token-a", wantEvents: `"events":[]`},
		{token: "token-b", wantEvents: `"id":"event-1"`},
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		request.Header.Set("Authorization", "Bearer "+check.token)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), check.wantEvents) {
			t.Fatalf("dashboard for %s = %d %s", check.token, response.Code, response.Body.String())
		}
	}
}

func TestServerReportsStorageAndProtectsProjectedWrites(t *testing.T) {
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "storage:read", "storage:write")), Dependencies{Dashboard: dashboard.NewService(1, nil, &audit.Log{}), Runners: runner.NewService("1.0.0"), Storage: storage.NewService(100, []storage.Usage{{Category: storage.CategoryLFSAssets, Bytes: 87}})})

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
	server := NewServer(auth.NewVerifier(auth.NewRecord("agent-1", "token", time.Time{}, "runners:write")), Dependencies{Dashboard: dashboard.NewService(1, nil, &audit.Log{}), Runners: runner.NewService("1.0.0"), Storage: storage.NewService(100, nil)})

	reconcile := httptest.NewRequest(http.MethodPost, "/api/v1/runners/reconcile", strings.NewReader(`{"id":"agent-1","platform":"windows/amd64"}`))
	reconcile.Header.Set("Authorization", "Bearer token")
	reconcile.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, reconcile)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"action":"install"`) {
		t.Fatalf("reconcile response = %d %s, want install action", response.Code, response.Body.String())
	}
}

func TestServerReadsAllowlistedRepositoryTree(t *testing.T) {
	repositories, err := repository.NewService([]repository.Repository{{ID: "gitsss", Name: "gitsss", Path: filepath.Join("..", "..", "..")}})
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(auth.NewVerifier(auth.NewRecord("user-1", "token", time.Time{}, "repositories:read")), Dependencies{Repositories: repositories})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/repositories/gitsss/tree?ref=HEAD", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"path":"server/go.mod"`) {
		t.Fatalf("tree response = %d %s, want tracked server/go.mod", response.Code, response.Body.String())
	}
}

func TestServerConfiguresAndTestsNotificationWithoutReturningSecrets(t *testing.T) {
	log := &audit.Log{}
	notifications := notification.NewService(notificationFactory{}, log)
	server := NewServer(auth.NewVerifier(auth.NewRecord("admin", "token", time.Time{}, "notifications:read", "notifications:write")), Dependencies{Notifications: notifications})

	configure := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/feishu", strings.NewReader(`{"webhook_url":"https://open.feishu.cn/open-apis/bot/v2/hook/private","secret":"private-secret"}`))
	configure.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, configure)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("configure response = %d %s, want masked response", response.Code, response.Body.String())
	}

	testRequest := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/feishu/test", strings.NewReader(`{"subject":"测试","body":"连接成功"}`))
	testRequest.Header.Set("Authorization", "Bearer token")
	testResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(testResponse, testRequest)
	if testResponse.Code != http.StatusOK || !strings.Contains(testResponse.Body.String(), `"status":"delivered"`) {
		t.Fatalf("test response = %d %s, want delivered", testResponse.Code, testResponse.Body.String())
	}
}

func TestServerHidesNotificationPersistenceErrors(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	vault, err := state.OpenVault(filepath.Join(root, "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	notifications, err := notification.NewPersistentService(notificationFactory{}, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	server := NewServer(auth.NewVerifier(auth.NewRecord("admin", "token", time.Time{}, "notifications:write")), Dependencies{Notifications: notifications})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/channels/feishu", strings.NewReader(`{"webhook_url":"https://open.feishu.cn/private","secret":"private-secret"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "closed") || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("persistence response = %d %s", response.Code, response.Body.String())
	}
}

func TestServerAcceptsAuditedTakeoverAssessment(t *testing.T) {
	log := &audit.Log{}
	server := NewServer(
		auth.NewVerifier(auth.NewRecord("agent-token", "token", time.Time{}, "takeover:read", "takeover:write")),
		Dependencies{Takeovers: takeover.NewRegistry(log)},
	)
	payload := `{"id":"assessment-0011223344556677","agent_id":"agent-1","config_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","database":{"type":"sqlite3","location":"local-file"},"assessed_at":"2026-07-14T12:00:00Z"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/takeover/assessments", strings.NewReader(payload))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || len(log.Entries()) != 1 {
		t.Fatalf("submit response/audit = %d %s / %d", response.Code, response.Body.String(), len(log.Entries()))
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/takeover/assessments", nil)
	list.Header.Set("Authorization", "Bearer token")
	listResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "assessment-0011223344556677") {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}
}

func TestServerDispatchesAgentJobWithScopedClaimAndCompletion(t *testing.T) {
	log := &audit.Log{}
	server := NewServer(
		auth.NewVerifier(
			auth.NewRecord("admin", "admin-token", time.Time{}, "agents:dispatch", "agents:read"),
			auth.NewRecord("worker", "worker-token", time.Time{}, "agents:work"),
		),
		Dependencies{Jobs: agentjob.NewQueue(log)},
	)
	create := httptest.NewRequest(http.MethodPost, "/api/v1/agent/jobs", strings.NewReader(`{"kind":"takeover.assess","input":{"agent_id":"old-server","config_path":"/etc/gitea/app.ini"}}`))
	create.Header.Set("Authorization", "Bearer admin-token")
	createResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", createResponse.Code, createResponse.Body.String())
	}
	var created agentjob.Job
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	claim := httptest.NewRequest(http.MethodPost, "/api/v1/agent/jobs/claim", strings.NewReader(`{"agent_id":"old-server"}`))
	claim.Header.Set("Authorization", "Bearer worker-token")
	claimResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(claimResponse, claim)
	if claimResponse.Code != http.StatusOK || !strings.Contains(claimResponse.Body.String(), created.ID) {
		t.Fatalf("claim = %d %s", claimResponse.Code, claimResponse.Body.String())
	}
	wrong := httptest.NewRequest(http.MethodPost, "/api/v1/agent/jobs/"+created.ID+"/complete", strings.NewReader(`{"agent_id":"other","result":{"id":"assessment-1"}}`))
	wrong.Header.Set("Authorization", "Bearer worker-token")
	wrongResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(wrongResponse, wrong)
	if wrongResponse.Code != http.StatusForbidden {
		t.Fatalf("wrong completion = %d %s", wrongResponse.Code, wrongResponse.Body.String())
	}
	complete := httptest.NewRequest(http.MethodPost, "/api/v1/agent/jobs/"+created.ID+"/complete", strings.NewReader(`{"agent_id":"old-server","result":{"id":"assessment-1"}}`))
	complete.Header.Set("Authorization", "Bearer worker-token")
	completeResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(completeResponse, complete)
	if completeResponse.Code != http.StatusOK || !strings.Contains(completeResponse.Body.String(), `"status":"succeeded"`) {
		t.Fatalf("complete = %d %s", completeResponse.Code, completeResponse.Body.String())
	}
	if len(log.Entries()) != 3 {
		t.Fatalf("audit entries = %#v", log.Entries())
	}
}

func TestServerAgentHealthAuthenticatesWithoutClaimingJob(t *testing.T) {
	log := &audit.Log{}
	queue := agentjob.NewQueue(log)
	if _, err := queue.Create("admin", "request-1", agentjob.KindAssessment, json.RawMessage(`{"agent_id":"old-server"}`), time.Now()); err != nil {
		t.Fatal(err)
	}
	server := NewServer(
		auth.NewVerifier(auth.NewRecord("worker", "worker-token", time.Time{}, "agents:work")),
		Dependencies{Jobs: queue},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/health", nil)
	request.Header.Set("Authorization", "Bearer worker-token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"token_id":"worker"`) {
		t.Fatalf("health = %d %s", response.Code, response.Body.String())
	}
	jobs := queue.Jobs()
	if len(jobs) != 1 || jobs[0].Status != agentjob.StatusQueued {
		t.Fatalf("jobs = %#v, want one queued job", jobs)
	}

	unauthorized := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/agent/health", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized health = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}
}
