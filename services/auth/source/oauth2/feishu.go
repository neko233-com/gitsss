// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"errors"
	"strings"

	"github.com/markbates/goth"
)

func feishuUserIdentity(user goth.User) (goth.User, error) {
	if strings.TrimSpace(user.UserID) != "" {
		return user, nil
	}
	// Basic identity grants expose open_id without the optional employee user_id.
	data, _ := user.RawData["data"].(map[string]any)
	openID, _ := data["open_id"].(string)
	user.UserID = strings.TrimSpace(openID)
	if user.UserID == "" {
		return user, errors.New("Feishu returned no usable login identity")
	}
	return user, nil
}
