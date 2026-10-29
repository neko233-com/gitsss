// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/tests"
	"github.com/stretchr/testify/assert"
)

func TestProfileRoleLabels(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	post := func(values map[string]string, status int) {
		values["name"] = "user2"
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings", values), status)
	}
	post(map[string]string{"role_labels": "程序， 美术, 自定义,程序"}, http.StatusSeeOther)
	page := session.MakeRequest(t, NewRequest(t, "GET", "/user/settings"), http.StatusOK).Body.String()
	assert.Contains(t, page, "程序, 美术, 自定义")
	menu := NewHTMLParser(t, strings.NewReader(page)).Find("#navbar .user-menu-roles")
	assert.Equal(t, 3, menu.Find(".user-menu-role").Length())
	assert.Equal(t, "程序美术自定义", menu.Text())
	otherProfile := session.MakeRequest(t, NewRequest(t, "GET", "/user1"), http.StatusOK).Body.String()
	assert.Equal(t, "程序美术自定义", NewHTMLParser(t, strings.NewReader(otherProfile)).Find("#navbar .user-menu-roles").Text())
	profile := MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.Contains(t, profile, ">程序</span>")
	assert.Contains(t, profile, ">自定义</span>")
	post(map[string]string{}, http.StatusSeeOther)
	profile = MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.Contains(t, profile, ">程序</span>")
	post(map[string]string{"role_labels": strings.Repeat("a", 33)}, http.StatusBadRequest)
	profile = MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.Contains(t, profile, ">程序</span>")
	post(map[string]string{"role_labels": ""}, http.StatusSeeOther)
	profile = MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.NotContains(t, profile, ">程序</span>")
	page = session.MakeRequest(t, NewRequest(t, "GET", "/user/settings"), http.StatusOK).Body.String()
	assert.Zero(t, NewHTMLParser(t, strings.NewReader(page)).Find("#navbar .user-menu-role").Length())
}

func TestAdminProfileRoleLabels(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	admin := loginUser(t, "user1")
	user := loginUser(t, "user2")
	endpoint := "/-/admin/users/2/role-labels"
	user.MakeRequest(t, NewRequestWithValues(t, "POST", endpoint, map[string]string{"role_labels": "bad"}), http.StatusForbidden)
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", endpoint, map[string]string{"role_labels": "客户端,后端"}), http.StatusSeeOther)
	edit := admin.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users/2/edit"), http.StatusOK).Body.String()
	assert.Contains(t, edit, "客户端, 后端")
	profile := user.MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.Contains(t, profile, ">客户端</span>")
	assert.Contains(t, profile, ">后端</span>")
	admin.MakeRequest(t, NewRequestWithValues(t, "POST", endpoint, map[string]string{"role_labels": strings.Repeat("a", 33)}), http.StatusBadRequest)
	profile = user.MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body.String()
	assert.Contains(t, profile, ">客户端</span>")
}
