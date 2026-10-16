// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration_test

import (
	"log"
	"testing"

	gitea "github.com/neko233-com/gitsss/packages/migration-sdk"

	"github.com/stretchr/testify/assert"
)

func TestUserSettings(t *testing.T) {
	log.Println("== TestUserSettings ==")
	c := newTestClient()

	userConf, _, err := c.Users.GetUserSettings(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, userConf)
	assert.EqualValues(t, gitea.UserSettings{
		Theme:        "gitea-auto",
		HideEmail:    false,
		HideActivity: false,
	}, *userConf)

	userConf, _, err = c.Users.UpdateUserSettings(t.Context(), gitea.UserSettingsOptions{
		FullName:  gitea.OptionalString("Admin User on Test"),
		Language:  gitea.OptionalString("de_de"),
		HideEmail: gitea.OptionalBool(true),
	})
	assert.NoError(t, err)
	assert.NotNil(t, userConf)
	assert.EqualValues(t, gitea.UserSettings{
		FullName:     "Admin User on Test",
		Theme:        "gitea-auto",
		Language:     "de_de",
		HideEmail:    true,
		HideActivity: false,
	}, *userConf)

	_, _, err = c.Users.UpdateUserSettings(t.Context(), gitea.UserSettingsOptions{
		FullName:  gitea.OptionalString(""),
		Language:  gitea.OptionalString(""),
		HideEmail: gitea.OptionalBool(false),
	})
	assert.NoError(t, err)
}
