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
	"github.com/neko233-com/gitsss/server/internal/importcatalog"
	"github.com/neko233-com/gitsss/server/internal/notification"
	"github.com/neko233-com/gitsss/server/internal/pipeline"
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
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), issued.Token) || strings.Contains(listResponse.Body.String(), `"hash"`) || !strings.Contains(listResponse.Body.String(), `"current":true`) {
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

func TestServerPublishesAuditedGitClientCatalogWithoutAuthentication(t *testing.T) {
	response := httptest.NewRecorder()
	NewServer(auth.NewVerifier(), Dependencies{}).Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/api/v1/git-clients", nil),
	)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"version":"2.55.0.windows.2"`) || !strings.Contains(response.Body.String(), `"kind":"mirror"`) || !strings.Contains(response.Body.String(), `"sha256":"74300da8`) {
		t.Fatalf("catalog = %d %s", response.Code, response.Body.String())
	}
}

func TestServerPublishesSanitizedImportedBusinessRecordsByScope(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Put("source-import-user", "00000000000000000001", json.RawMessage(`{"source_table":"user","ordinal":1,"values":{"id":1,"name":"artist","passwd":"secret-hash"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("source-import-issue", "00000000000000000001", json.RawMessage(`{"source_table":"issue","ordinal":1,"values":{"id":2,"repo_id":3,"index":4,"name":"Review"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("import-work-items", "00000000000000000003-00000000000000000001", json.RawMessage(`{"source_table":"issue","ordinal":1,"values":{"id":2,"repo_id":3,"index":4,"name":"Review"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("import-repository-history", "00000000000000000003-00000000000000000001", json.RawMessage(`{"source_table":"release","ordinal":1,"values":{"id":5,"_repository_id":3,"tag_name":"v1.0.0","name":"First release"}}`)); err != nil {
		t.Fatal(err)
	}
	catalog, err := importcatalog.New(store)
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifier(
		auth.NewRecord("admin", "admin-token", time.Time{}, "auth:read"),
		auth.NewRecord("reader", "reader-token", time.Time{}, "repositories:read"),
	)
	handler := NewServer(verifier, Dependencies{Imports: catalog}).Handler()
	directory := httptest.NewRequest(http.MethodGet, "/api/v1/imports/directory?limit=1", nil)
	directory.Header.Set("Authorization", "Bearer admin-token")
	directoryResponse := httptest.NewRecorder()
	handler.ServeHTTP(directoryResponse, directory)
	if directoryResponse.Code != http.StatusOK || !strings.Contains(directoryResponse.Body.String(), `"name":"artist"`) || strings.Contains(directoryResponse.Body.String(), "secret-hash") {
		t.Fatalf("directory = %d %s", directoryResponse.Code, directoryResponse.Body.String())
	}
	items := httptest.NewRequest(http.MethodGet, "/api/v1/imports/work-items?repository_id=3", nil)
	items.Header.Set("Authorization", "Bearer reader-token")
	itemsResponse := httptest.NewRecorder()
	handler.ServeHTTP(itemsResponse, items)
	if itemsResponse.Code != http.StatusOK || !strings.Contains(itemsResponse.Body.String(), `"title":"Review"`) {
		t.Fatalf("work items = %d %s", itemsResponse.Code, itemsResponse.Body.String())
	}
	history := httptest.NewRequest(http.MethodGet, "/api/v1/imports/repository-history?repository_id=3", nil)
	history.Header.Set("Authorization", "Bearer reader-token")
	historyResponse := httptest.NewRecorder()
	handler.ServeHTTP(historyResponse, history)
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"kind":"release"`) || !strings.Contains(historyResponse.Body.String(), `"tag":"v1.0.0"`) {
		t.Fatalf("repository history = %d %s", historyResponse.Code, historyResponse.Body.String())
	}
	deniedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/imports/directory", nil)
	deniedRequest.Header.Set("Authorization", "Bearer reader-token")
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, deniedRequest)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("directory reader status = %d", denied.Code)
	}
}

func TestServerPreviewsRoleAndIssuesRoleToken(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	verifier := auth.NewVerifier(auth.NewRecord("admin", "token", time.Time{}, "auth:read", "auth:write"))
	server := NewServer(verifier, Dependencies{})
	server.now = func() time.Time { return now }

	preview := httptest.NewRequest(http.MethodGet, "/api/v1/auth/roles/contributor/preview", nil)
	preview.Header.Set("Authorization", "Bearer token")
	previewResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(previewResponse, preview)
	if previewResponse.Code != http.StatusOK || !strings.Contains(previewResponse.Body.String(), `"maximum_hours":2160`) || !strings.Contains(previewResponse.Body.String(), `"repositories:push"`) {
		t.Fatalf("role preview = %d %s", previewResponse.Code, previewResponse.Body.String())
	}

	issue := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"id":"artist","role":"contributor"}`))
	issue.Header.Set("Authorization", "Bearer token")
	issueResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(issueResponse, issue)
	if issueResponse.Code != http.StatusCreated || !strings.Contains(issueResponse.Body.String(), `"role":"contributor"`) || !strings.Contains(issueResponse.Body.String(), `"token":"gitsss_`) {
		t.Fatalf("role issue = %d %s", issueResponse.Code, issueResponse.Body.String())
	}

	overlong := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"id":"overlong","role":"contributor","expires_in_hours":2161}`))
	overlong.Header.Set("Authorization", "Bearer token")
	overlongResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(overlongResponse, overlong)
	if overlongResponse.Code != http.StatusBadRequest {
		t.Fatalf("overlong role issue status = %d", overlongResponse.Code)
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
	plan := httptest.NewRequest(http.MethodGet, "/api/v1/storage/cleanup-plan", nil)
	plan.Header.Set("Authorization", "Bearer token")
	planResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(planResponse, plan)
	if planResponse.Code != http.StatusOK || !strings.Contains(planResponse.Body.String(), `"status":"warning"`) || !strings.Contains(planResponse.Body.String(), `"bytes_to_target":2`) || !strings.Contains(planResponse.Body.String(), `"automatic_delete":false`) {
		t.Fatalf("cleanup plan response = %d %s", planResponse.Code, planResponse.Body.String())
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

func TestServerRunsPipelineThroughBoundRunnerIdentity(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	pipelines, err := pipeline.NewPersistentService(log, store)
	if err != nil {
		t.Fatal(err)
	}
	repositories, err := repository.NewService([]repository.Repository{{ID: "gitsss", Name: "gitsss", Path: filepath.Join("..", "..", "..")}})
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifier(
		auth.NewRecord("maintainer", "maintainer-token", time.Time{}, "dashboard:read", "pipelines:read", "pipelines:write"),
		auth.NewRecord("runner-1", "runner-token", time.Time{}, "pipelines:work"),
	)
	notifications := notification.NewService(notificationFactory{}, log)
	if _, err := notifications.Configure("maintainer", "configure", notification.Configuration{Channel: notification.ChannelFeishu}); err != nil {
		t.Fatal(err)
	}
	dashboardService := dashboard.NewService(6, nil, log)
	server := NewServer(verifier, Dependencies{Audit: log, Dashboard: dashboardService, Notifications: notifications, Pipelines: pipelines, Repositories: repositories})
	server.now = func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	create := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs", strings.NewReader(`{"repository_id":"gitsss","ref":"HEAD","template_id":"gitsss-verify"}`))
	create.Header.Set("Authorization", "Bearer maintainer-token")
	createResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("pipeline create = %d %s", createResponse.Code, createResponse.Body.String())
	}
	var created pipeline.Run
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	dashboardRequest := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	dashboardRequest.Header.Set("Authorization", "Bearer maintainer-token")
	dashboardResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(dashboardResponse, dashboardRequest)
	if dashboardResponse.Code != http.StatusOK || !strings.Contains(dashboardResponse.Body.String(), `"label":"运行中","value":1`) || !strings.Contains(dashboardResponse.Body.String(), `"id":"pipeline:`+created.ID) {
		t.Fatalf("live dashboard = %d %s", dashboardResponse.Code, dashboardResponse.Body.String())
	}

	wrongClaim := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs/claim", strings.NewReader(`{"runner_id":"other"}`))
	wrongClaim.Header.Set("Authorization", "Bearer runner-token")
	wrongClaimResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(wrongClaimResponse, wrongClaim)
	if wrongClaimResponse.Code != http.StatusForbidden {
		t.Fatalf("wrong claim status = %d", wrongClaimResponse.Code)
	}
	claim := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs/claim", strings.NewReader(`{"runner_id":"runner-1"}`))
	claim.Header.Set("Authorization", "Bearer runner-token")
	claimResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(claimResponse, claim)
	if claimResponse.Code != http.StatusOK || !strings.Contains(claimResponse.Body.String(), `"template":{"id":"gitsss-verify"`) {
		t.Fatalf("pipeline claim = %d %s", claimResponse.Code, claimResponse.Body.String())
	}

	heartbeat := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs/"+created.ID+"/heartbeat", strings.NewReader(`{"runner_id":"runner-1","log":"tests passed\n"}`))
	heartbeat.Header.Set("Authorization", "Bearer runner-token")
	heartbeatResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(heartbeatResponse, heartbeat)
	if heartbeatResponse.Code != http.StatusOK {
		t.Fatalf("pipeline heartbeat = %d %s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}

	complete := httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/runs/"+created.ID+"/complete", strings.NewReader(`{"runner_id":"runner-1","succeeded":true}`))
	complete.Header.Set("Authorization", "Bearer runner-token")
	completeResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(completeResponse, complete)
	if completeResponse.Code != http.StatusOK || !strings.Contains(completeResponse.Body.String(), `"status":"succeeded"`) {
		t.Fatalf("pipeline complete = %d %s", completeResponse.Code, completeResponse.Body.String())
	}
	if messages := notifications.Messages(); len(messages) != 1 || messages[0].Status != notification.StatusPending || !strings.Contains(messages[0].Body, created.ID) {
		t.Fatalf("pipeline notifications = %#v", messages)
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

func TestServerCreatesPersistentHostedRepository(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	repositories, err := repository.NewPersistentService(log, store, filepath.Join(root, "repositories"), nil)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(auth.NewVerifier(auth.NewRecord("admin", "token", time.Time{}, "repositories:write")), Dependencies{Repositories: repositories, Audit: log})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"id":"art-team","name":"Art Team","default_branch":"main"}`))
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"hosted":true`) || strings.Contains(response.Body.String(), filepath.Join(root, "repositories")) {
		t.Fatalf("create response = %d %s", response.Code, response.Body.String())
	}
	if _, err := repositories.Resolve("art-team"); err != nil {
		t.Fatal(err)
	}

	malformed := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"id":"bad","name":"Bad"}`))
	malformed.Header.Set("Authorization", "Bearertoken")
	malformedResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(malformedResponse, malformed)
	if malformedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("malformed bearer status = %d", malformedResponse.Code)
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
			auth.NewRecord("old-server", "worker-token", time.Time{}, "agents:work"),
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
	heartbeat := httptest.NewRequest(http.MethodPost, "/api/v1/agent/jobs/"+created.ID+"/heartbeat", strings.NewReader(`{"agent_id":"old-server"}`))
	heartbeat.Header.Set("Authorization", "Bearer worker-token")
	heartbeatResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(heartbeatResponse, heartbeat)
	if heartbeatResponse.Code != http.StatusOK || !strings.Contains(heartbeatResponse.Body.String(), `"status":"running"`) {
		t.Fatalf("heartbeat = %d %s", heartbeatResponse.Code, heartbeatResponse.Body.String())
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
