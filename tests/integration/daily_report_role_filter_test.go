// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/dailyreport"
	"github.com/neko233-com/gitsss/modules/setting"
	dailyreport_service "github.com/neko233-com/gitsss/services/dailyreport"
	"github.com/neko233-com/gitsss/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyReportRoleFilterSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	admin := loginUser(t, "user1")
	require.NoError(t, user_model.SetUserSettingJSON(t.Context(), 2, user_model.SettingsKeyRoleLabels, []string{"客户端", "策划"}))
	oldPath := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	t.Cleanup(func() { setting.AppDataPath = oldPath })
	require.NoError(t, dailyreport_service.SaveConfig(dailyreport.Config{Schedule: dailyreport.Schedule{Time: "21:00"}, Accounts: map[string]string{}}))
	page := admin.MakeRequest(t, NewRequest(t, "GET", "/-/admin/daily-report"), http.StatusOK).Body.String()
	assert.Contains(t, page, "客户端")
	assert.Contains(t, page, "策划")
	post := func(mode string, roles ...string) int {
		values := url.Values{"time": {"21:00"}, "role_filter_mode": {mode}, "role_filter": roles}
		req := NewRequestWithURLValues(t, "POST", "/-/admin/daily-report", values)
		expected := http.StatusSeeOther
		if mode == "selected" && len(roles) == 0 {
			expected = http.StatusBadRequest
		}
		return admin.MakeRequest(t, req, expected).Code
	}
	assert.Equal(t, http.StatusSeeOther, post("selected", "客户端", "策划"))
	cfg, err := dailyreport_service.LoadConfig()
	require.NoError(t, err)
	assert.True(t, cfg.RoleFilterEnabled)
	assert.Equal(t, []string{"客户端", "策划"}, cfg.RoleLabels)
	assert.Equal(t, http.StatusBadRequest, post("selected"))
	cfg, err = dailyreport_service.LoadConfig()
	require.NoError(t, err)
	assert.Len(t, cfg.RoleLabels, 2)
	assert.Equal(t, http.StatusSeeOther, post("all"))
	cfg, err = dailyreport_service.LoadConfig()
	require.NoError(t, err)
	assert.False(t, cfg.RoleFilterEnabled)
}
