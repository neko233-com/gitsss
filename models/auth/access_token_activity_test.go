// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/unittest"
	"github.com/neko233-com/gitsss/modules/timeutil"
	"github.com/neko233-com/gitsss/modules/util"

	"github.com/stretchr/testify/require"
)

func TestAccessTokenActivityPreservesCurrentState(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	const value = "d2c6c1ba3890b309189a8e618c72a162e4efbf36"
	stale, err := auth_model.GetAccessTokenBySHA(t.Context(), value)
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(stale.ID).Cols("name", "scope").Update(&auth_model.AccessToken{Name: "renamed", Scope: "read:repository"})
	require.NoError(t, err)
	require.NoError(t, auth_model.TouchAccessToken(t.Context(), stale))
	actual := unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{ID: stale.ID})
	require.Equal(t, "renamed", actual.Name)
	require.EqualValues(t, "read:repository", actual.Scope)
	require.GreaterOrEqual(t, actual.UpdatedUnix, timeutil.TimeStampNow()-1)
	cached, err := auth_model.GetAccessTokenBySHA(t.Context(), value)
	require.NoError(t, err)
	require.Equal(t, actual.Scope, cached.Scope)
	require.NoError(t, auth_model.TouchAccessToken(t.Context(), cached))
	require.NoError(t, auth_model.DeleteAccessTokenByID(t.Context(), stale.ID, stale.UID))
	require.NoError(t, auth_model.TouchAccessToken(t.Context(), stale))
	_, err = auth_model.GetAccessTokenBySHA(t.Context(), value)
	require.ErrorIs(t, err, util.ErrNotExist)
	unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: stale.ID})
}
