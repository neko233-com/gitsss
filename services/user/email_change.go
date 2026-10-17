// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"errors"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/util"
)

// ChangePrimaryEmail keeps the old address as an alias so existing commits retain their attribution.
// With email confirmation enabled the new address must be activated before it can become primary.
func ChangePrimaryEmail(ctx context.Context, user *user_model.User, address string) (pending bool, err error) {
	address = strings.TrimSpace(address)
	if err := user_model.ValidateEmail(address); err != nil {
		return false, err
	}
	err = db.WithTx(ctx, func(ctx context.Context) error {
		email, lookupErr := user_model.GetEmailAddressByEmail(ctx, address)
		if lookupErr != nil && !errors.Is(lookupErr, util.ErrNotExist) {
			return lookupErr
		}
		if email != nil && email.UID != user.ID {
			return user_model.ErrEmailAlreadyUsed{Email: address}
		}
		if email == nil {
			if err := AddEmailAddresses(ctx, user, []string{address}); err != nil {
				return err
			}
			email, lookupErr = user_model.GetEmailAddressByEmail(ctx, address)
			if lookupErr != nil {
				return lookupErr
			}
		}
		if !email.IsActivated {
			pending = true
			return nil
		}
		return user_model.MakeActiveEmailPrimary(ctx, user.ID, email.ID)
	})
	if err != nil {
		if existing, lookupErr := user_model.GetEmailAddressByEmail(ctx, address); lookupErr == nil && existing.UID != user.ID {
			return false, user_model.ErrEmailAlreadyUsed{Email: address}
		}
	}
	return pending, err
}
