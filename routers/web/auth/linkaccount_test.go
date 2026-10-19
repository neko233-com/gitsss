// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestLinkAccountDefaultsToExistingAccount(t *testing.T) {
	for _, disabled := range []bool{true, false} {
		for _, internalOnly := range []bool{true, false} {
			func() {
				defer test.MockVariableValue(&setting.Service.DisableRegistration, disabled)()
				defer test.MockVariableValue(&setting.Service.AllowOnlyInternalRegistration, internalOnly)()
				ctx, _ := contexttest.MockContext(t, "/user/link_account")
				prepareLinkAccountPageData(ctx)
				assert.Equal(t, true, ctx.Data["LinkAccountModeSignIn"])
				assert.Equal(t, !disabled && !internalOnly, ctx.Data["LinkAccountRegistrationAllowed"])
			}()
		}
	}
}
