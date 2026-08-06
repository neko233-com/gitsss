// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

type loginFailureCache interface {
	Get(key string) (string, bool)
	Put(key, value string, ttl int64) error
	Delete(key string) error
}

const loginFailureTTL = int64((15 * time.Minute) / time.Second)

// RecordLoginFailure slows repeated failures for an account and the source address.
// It never blocks a successful password, so an account cannot be locked by guesses.
func RecordLoginFailure(ctx context.Context, c loginFailureCache, username, remoteAddr string) time.Duration {
	if c == nil {
		return 0
	}

	accountFailures := bumpLoginFailure(c, loginFailureKey("account", strings.ToLower(strings.TrimSpace(username))))
	ipFailures := bumpLoginFailure(c, loginFailureKey("ip", remoteAddr))
	return loginFailureDelay(max(accountFailures, ipFailures))
}

// ClearLoginFailures removes the failure backoff after a successful password check.
func ClearLoginFailures(ctx context.Context, c loginFailureCache, username, remoteAddr string) {
	if c == nil {
		return
	}
	_ = c.Delete(loginFailureKey("account", strings.ToLower(strings.TrimSpace(username))))
	_ = c.Delete(loginFailureKey("ip", remoteAddr))
}

func loginFailureKey(scope, value string) string {
	hash := sha256.Sum256([]byte(value))
	return "LoginFailure_" + scope + "_" + hex.EncodeToString(hash[:])
}

func bumpLoginFailure(c loginFailureCache, key string) int {
	count := 0
	if value, ok := c.Get(key); ok {
		count, _ = strconv.Atoi(value)
	}
	count++
	_ = c.Put(key, strconv.Itoa(count), loginFailureTTL)
	return count
}

func loginFailureDelay(count int) time.Duration {
	if count <= 3 {
		return 0
	}
	shift := min(count-4, 4)
	return min(5*time.Second, 250*time.Millisecond*time.Duration(1<<shift))
}
