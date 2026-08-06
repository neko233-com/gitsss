// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/timeutil"
	"github.com/neko233-com/gitsss/modules/util"

	"xorm.io/builder"
)

var ErrAuthTokenNotExist = util.NewNotExistErrorf("auth token does not exist")

type AuthToken struct { //nolint:revive // export stutter
	ID          string `xorm:"pk"`
	TokenHash   string
	UserID      int64              `xorm:"INDEX"`
	ExpiresUnix timeutil.TimeStamp `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(AuthToken))
}

func InsertAuthToken(ctx context.Context, t *AuthToken) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}

func GetAuthTokenByID(ctx context.Context, id string) (*AuthToken, error) {
	at := &AuthToken{}

	has, err := db.GetEngine(ctx).ID(id).Get(at)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrAuthTokenNotExist
	}
	return at, nil
}

func UpdateAuthTokenByID(ctx context.Context, t *AuthToken) error {
	_, err := db.GetEngine(ctx).ID(t.ID).Cols("token_hash", "expires_unix").Update(t)
	return err
}

func DeleteAuthTokenByID(ctx context.Context, id string) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&AuthToken{})
	return err
}

// ConsumeAuthTokenByID atomically invalidates a token when its current hash matches.
func ConsumeAuthTokenByID(ctx context.Context, id, tokenHash string) (bool, error) {
	count, err := db.GetEngine(ctx).ID(id).
		Where("token_hash = ? AND expires_unix >= ?", tokenHash, timeutil.TimeStampNow()).
		Cols("token_hash").Update(&AuthToken{TokenHash: ""})
	return count == 1, err
}

// ListAccountInviteTokens returns pending account setup invitations.
func ListAccountInviteTokens(ctx context.Context) ([]*AuthToken, error) {
	var tokens []*AuthToken
	err := db.GetEngine(ctx).
		Where("user_id = ? AND id LIKE ? AND token_hash <> '' AND expires_unix >= ?", 0, "invite_%", timeutil.TimeStampNow()).
		Desc("expires_unix").Find(&tokens)
	return tokens, err
}

func DeleteAuthTokensByUserID(ctx context.Context, uid int64) error {
	_, err := db.GetEngine(ctx).Where(builder.Eq{"user_id": uid}).Delete(&AuthToken{})
	return err
}

func DeleteExpiredAuthTokens(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where(builder.Lt{"expires_unix": timeutil.TimeStampNow()}).Delete(&AuthToken{})
	return err
}
