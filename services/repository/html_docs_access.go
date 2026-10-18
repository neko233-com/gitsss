// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
)

// HTMLDocsAccess is a short-lived read capability for opaque sandbox asset requests.
type HTMLDocsAccess struct {
	UserID   int64  `json:"user"`
	RepoID   int64  `json:"repo"`
	Revision int64  `json:"revision"`
	Commit   string `json:"commit"`
	Expires  int64  `json:"expires"`
	Preview  bool   `json:"preview,omitempty"`
}

func SignHTMLDocsAccess(access HTMLDocsAccess) (string, error) {
	body, err := json.Marshal(access)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(setting.SecretKey))
	_, _ = mac.Write([]byte("gitsss-html-docs-v1:" + encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyHTMLDocsAccess(token string, now time.Time) (*HTMLDocsAccess, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 1024 {
		return nil, errors.New("invalid documentation access")
	}
	mac := hmac.New(sha256.New, []byte(setting.SecretKey))
	_, _ = mac.Write([]byte("gitsss-html-docs-v1:" + parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), signature) {
		return nil, errors.New("invalid documentation access")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var access HTMLDocsAccess
	if err := json.Unmarshal(body, &access); err != nil {
		return nil, err
	}
	if access.RepoID <= 0 || access.UserID < 0 || access.Expires <= now.Unix() || access.Expires > now.Add(time.Hour).Unix() {
		return nil, errors.New("expired documentation access")
	}
	return &access, nil
}
