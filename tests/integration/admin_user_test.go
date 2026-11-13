// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/organization"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/assert"
)

func TestAdminViewUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/users")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/-/admin/users")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPISafeDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	admin := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin)
	user := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteAdmin)
	request := func(name, token string, status int) {
		req := NewRequest(t, "DELETE", "/api/v1/admin/users/"+name+"/safe").AddTokenAuth(token)
		MakeRequest(t, req, status)
	}
	request("user4", user, http.StatusForbidden)
	request("user1", admin, http.StatusUnprocessableEntity)
	request("user5", admin, http.StatusUnprocessableEntity)
	request("user4", admin, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &user_model.User{ID: 4})
	unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: 4, OrgID: 3})
}

func TestAdminUserListSearchAndActions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users?q=EIGhT"), http.StatusOK)
	body := resp.Body.String()
	assert.Contains(t, body, "User Eight")
	assert.Contains(t, body, `id="delete-user-8"`)
	assert.Contains(t, body, `action="/-/admin/users/8/delete"`)
	allUsers := session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/users"), http.StatusOK).Body.String()
	assert.NotContains(t, allUsers, `id="delete-user-1"`)
	assert.Regexp(t, `class="admin-user-date">\d{4}-\d{2}-\d{2}</td>`, allUsers)
}

func TestAdminDashboardStorageLoadsSeparately(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/-/admin"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), `data-fetch-url="/-/admin/dashboard_storage"`)
	assert.NotContains(t, resp.Body.String(), `class="storage-dashboard-stats"`)
	session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/dashboard_storage"), http.StatusOK)
	loginUser(t, "user2").MakeRequest(t, NewRequest(t, "GET", "/-/admin/dashboard_storage"), http.StatusForbidden)
}

func TestAdminViewUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/users/1")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/-/admin/users/1")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminEditUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testSuccessfulEdit(t, user_model.User{ID: 2, Name: "newusername", LoginName: "otherlogin", Email: "new@e-mail.gitea"})
}

func testSuccessfulEdit(t *testing.T, formData user_model.User) {
	makeRequest(t, formData, http.StatusSeeOther)
}

func makeRequest(t *testing.T, formData user_model.User, headerCode int) {
	session := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", "/-/admin/users/"+strconv.Itoa(int(formData.ID))+"/edit", map[string]string{
		"user_name":  formData.Name,
		"login_name": formData.LoginName,
		"login_type": "0-0",
		"email":      formData.Email,
	})

	session.MakeRequest(t, req, headerCode)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: formData.ID})
	assert.Equal(t, formData.Name, user.Name)
	assert.Equal(t, formData.LoginName, user.LoginName)
	assert.Equal(t, formData.Email, user.Email)
}

func TestAdminDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	usersToDelete := []struct {
		userID int64
		purge  bool
	}{
		{
			userID: 2,
			purge:  true,
		},
		{
			userID: 8,
		},
	}

	for _, entry := range usersToDelete {
		t.Run(fmt.Sprintf("DeleteUser%d", entry.userID), func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: entry.userID})
			assert.NotNil(t, user)

			var query string
			if entry.purge {
				query = "?purge=true"
			}

			req := NewRequest(t, "POST", fmt.Sprintf("/-/admin/users/%d/delete%s", entry.userID, query))
			session.MakeRequest(t, req, http.StatusSeeOther)

			assertUserDeleted(t, entry.userID)
			unittest.CheckConsistencyFor(t, &user_model.User{})
		})
	}
}
