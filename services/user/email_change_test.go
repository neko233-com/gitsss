// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangePrimaryEmailUniqueAndPreservesAliases(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Service.RegisterEmailConfirm, false)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	old := user.Email
	pending, err := ChangePrimaryEmail(t.Context(), user, "new-primary@example.com")
	require.NoError(t, err)
	assert.False(t, pending)
	primary, err := user_model.GetPrimaryEmailAddressOfUser(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-primary@example.com", primary.Email)
	alias, err := user_model.GetEmailAddressByEmail(t.Context(), old)
	require.NoError(t, err)
	assert.Equal(t, user.ID, alias.UID)
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
	_, err = ChangePrimaryEmail(t.Context(), other, "NEW-PRIMARY@example.com")
	assert.True(t, user_model.IsErrEmailAlreadyUsed(err))
	_, err = ChangePrimaryEmail(t.Context(), other, old)
	assert.True(t, user_model.IsErrEmailAlreadyUsed(err))
	_, err = ChangePrimaryEmail(t.Context(), user, "invalid")
	assert.Error(t, err)
	_, err = ChangePrimaryEmail(t.Context(), user, old)
	require.NoError(t, err)
	primary, err = user_model.GetPrimaryEmailAddressOfUser(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, old, primary.Email)
}

func TestChangePrimaryEmailConfirmation(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Service.RegisterEmailConfirm, true)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	old := user.Email
	pending, err := ChangePrimaryEmail(t.Context(), user, "confirm-new@example.com")
	require.NoError(t, err)
	assert.True(t, pending)
	primary, err := user_model.GetPrimaryEmailAddressOfUser(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, old, primary.Email)
}
