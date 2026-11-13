// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/organization"
	packages_model "github.com/neko233-com/gitsss/models/packages"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"

	"github.com/stretchr/testify/require"
)

func TestSafeDeleteUser(t *testing.T) {
	t.Run("member", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, SafeDeleteUser(t.Context(), u))
		unittest.AssertNotExistsBean(t, &user_model.User{ID: 4})
		unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: 4, OrgID: 3})
		unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: 4, OrgID: 3})
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	})
	t.Run("owned repository", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		require.True(t, repo_model.IsErrUserOwnRepos(SafeDeleteUser(t.Context(), u)))
		unittest.AssertExistsAndLoadBean(t, &organization.OrgUser{UID: 5, OrgID: 6})
	})
	t.Run("owned package", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, db.Insert(t.Context(), &packages_model.Package{OwnerID: 4, Type: "generic", Name: "test", LowerName: "test"}))
		require.True(t, packages_model.IsErrUserOwnPackages(SafeDeleteUser(t.Context(), u)))
		unittest.AssertExistsAndLoadBean(t, &organization.OrgUser{UID: 4, OrgID: 3})
	})
	t.Run("last organization owner", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		_, err := db.Exec(t.Context(), "UPDATE team_user SET uid = ? WHERE id = ?", 24, 26)
		require.NoError(t, err)
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 24})
		err = SafeDeleteUser(t.Context(), u)
		require.True(t, organization.IsErrLastOrgOwner(err), "%v", err)
		unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 24})
		unittest.AssertExistsAndLoadBean(t, &organization.OrgUser{UID: 24, OrgID: 25})
	})
}
