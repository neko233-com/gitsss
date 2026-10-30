// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	activities_model "github.com/neko233-com/gitsss/models/activities"
	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/repository"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/timeutil"
	dailyreport_service "github.com/neko233-com/gitsss/services/dailyreport"
	"github.com/neko233-com/gitsss/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDailyReportRoleFilterSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	owner := loginUser(t, "user2")
	member := loginUser(t, "user4")
	admin := loginUser(t, "user1")
	require.NoError(t, user_model.SetUserSettingJSON(t.Context(), 2, user_model.SettingsKeyRoleLabels, []string{"客户端", "策划"}))
	oldPath := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	t.Cleanup(func() { setting.AppDataPath = oldPath })
	require.NoError(t, dailyreport_service.SaveOrgConfig(3, dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, Accounts: map[string]string{}}))
	page := owner.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings/daily-report"), http.StatusOK).Body.String()
	assert.Contains(t, page, "客户端")
	assert.Contains(t, page, "策划")
	assert.NotContains(t, page, "user5")
	member.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings/daily-report"), http.StatusNotFound)
	admin.MakeRequest(t, NewRequest(t, "GET", "/-/admin/daily-report"), http.StatusGone)
	post := func(mode string, status int, roles ...string) {
		values := url.Values{"time": {"21:00"}, "role_filter_mode": {mode}, "role_filter": roles}
		owner.MakeRequest(t, NewRequestWithURLValues(t, "POST", "/org/org3/settings/daily-report", values), status)
	}
	post("selected", http.StatusSeeOther, "客户端", "策划")
	cfg, err := dailyreport_service.LoadOrgConfig(3)
	require.NoError(t, err)
	assert.True(t, cfg.RoleFilterEnabled)
	assert.Equal(t, []string{"客户端", "策划"}, cfg.RoleLabels)
	other, err := dailyreport_service.LoadOrgConfig(6)
	require.NoError(t, err)
	assert.False(t, other.Schedule.Enabled)
	assert.Empty(t, other.RoleLabels)
	post("selected", http.StatusBadRequest)
	post("all", http.StatusSeeOther)
}

func TestOrganizationDailyReportAnalysisIsolation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	oldPath := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	t.Cleanup(func() { setting.AppDataPath = oldPath })
	require.NoError(t, user_model.SetUserSettingJSON(t.Context(), 4, user_model.SettingsKeyRoleLabels, []string{"策划"}))
	now := time.Now()
	var payloads []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Input string `json:"input"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
		payloads = append(payloads, input.Input)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"summary"}]}]}`)
	}))
	defer server.Close()
	previousTransport := http.DefaultTransport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // test server certificate
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	cfg := dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, BaseURL: server.URL, Model: "test", APIKey: "test", Accounts: map[string]string{"user2": "person", "user4": "person"}}
	require.NoError(t, dailyreport_service.SaveOrgConfig(3, cfg))
	add := func(repoID, authorID int64, email, message string) {
		raw, err := json.Marshal(repository.PushCommits{Len: 1, Commits: []*repository.PushCommit{{Sha1: fmt.Sprintf("%040d", repoID*100+authorID), AuthorEmail: email, Message: message}}})
		require.NoError(t, err)
		require.NoError(t, db.Insert(t.Context(), &activities_model.Action{UserID: 2, ActUserID: 2, RepoID: repoID, OpType: activities_model.ActionCommitRepo, Content: string(raw), CreatedUnix: timeutil.TimeStamp(now.Unix())}))
	}
	add(3, 2, "user2@example.com", "ORG_ONE")
	add(3, 4, "user4@example.com", "ORG_TWO")
	add(1, 2, "user2@example.com", "OTHER_REPOSITORY")
	add(23, 2, "user2@example.com", "OTHER_ORGANIZATION")
	results, err := dailyreport_service.AnalyzeOrganization(t.Context(), 3, now, dailyreport_service.ManualSelection{AccountIDs: []int64{2}, RoleLabels: []string{"策划"}})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "user2", results[0].Account)
	assert.Equal(t, "user4", results[1].Account)
	assert.Contains(t, strings.Join(payloads, " "), "ORG_ONE")
	assert.Contains(t, strings.Join(payloads, " "), "ORG_TWO")
	assert.NotContains(t, strings.Join(payloads, " "), "OTHER_REPOSITORY")
	assert.NotContains(t, strings.Join(payloads, " "), "OTHER_ORGANIZATION")
	_, err = dailyreport_service.AnalyzeOrganization(t.Context(), 3, now, dailyreport_service.ManualSelection{})
	assert.Error(t, err)
	_, err = dailyreport_service.AnalyzeOrganization(t.Context(), 3, now, dailyreport_service.ManualSelection{AccountIDs: []int64{5}})
	assert.Error(t, err)

	// Scheduled runs use organization-scoped receipts and never send twice.
	sent := 0
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "open.feishu.cn" {
			sent++
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":0}`)), Header: make(http.Header)}, nil
		}
		return transport.RoundTrip(r)
	})
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	cfg.Webhook = "https://open.feishu.cn/open-apis/bot/v2/hook/test"
	cfg.Schedule.Enabled = true
	cfg.Schedule.Time = now.In(loc).Format("15:04")
	require.NoError(t, dailyreport_service.SaveOrgConfig(3, cfg))
	run, err := dailyreport_service.ExecuteOrganization(t.Context(), 3, now)
	require.NoError(t, err)
	assert.Equal(t, 2, run.Sent)
	assert.Equal(t, 2, sent)
	run, err = dailyreport_service.ExecuteOrganization(t.Context(), 3, now)
	require.NoError(t, err)
	assert.Equal(t, 2, run.Skipped)
	assert.Equal(t, 2, sent)
	require.NoError(t, dailyreport_service.SaveOrgConfig(6, cfg))
	run, err = dailyreport_service.ExecuteOrganization(t.Context(), 6, now)
	require.NoError(t, err)
	assert.Equal(t, 0, run.Sent)
	assert.Equal(t, 2, sent)
	require.NoError(t, dailyreport_service.SaveOrgConfig(17, cfg))
	run, err = dailyreport_service.ExecuteOrganization(t.Context(), 17, now)
	require.NoError(t, err)
	assert.Equal(t, 1, run.Sent)
	assert.Equal(t, 3, sent)
	run, err = dailyreport_service.ExecuteOrganization(t.Context(), 17, now)
	require.NoError(t, err)
	assert.Equal(t, 1, run.Skipped)
	assert.Equal(t, 3, sent)
}
