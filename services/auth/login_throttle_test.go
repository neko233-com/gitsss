// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginGuardSharedNAT(t *testing.T) {
	g := newLoginGuard()
	for range 5 {
		finish, err := g.begin("attacked", "wrong", "192.0.2.1:1000", true, false)
		require.NoError(t, err)
		finish(false)
	}
	assert.True(t, g.challenge("attacked", "correct", "192.0.2.1:2000"))
	assert.False(t, g.challenge("colleague", "correct", "192.0.2.1:2000"))
	finish, err := g.begin("colleague", "correct", "192.0.2.1:2000", true, false)
	require.NoError(t, err)
	finish(true)
	// Solving a challenge admits even the targeted account; there is no account lock.
	finish, err = g.begin("attacked", "correct", "192.0.2.1:2000", true, true)
	require.NoError(t, err)
	finish(true)
	assert.False(t, g.challenge("attacked", "correct", "192.0.2.1:2000"))
}

func TestLoginGuardSprayAndSuccessDoNotClearSource(t *testing.T) {
	g := newLoginGuard()
	for i := range 30 {
		finish, err := g.begin(fmt.Sprint("unknown", i), "wrong", "192.0.2.2", true, false)
		require.NoError(t, err)
		finish(false)
	}
	assert.True(t, g.challenge("colleague", "correct", "192.0.2.2"))
	finish, err := g.begin("colleague", "correct", "192.0.2.2", true, true)
	require.NoError(t, err)
	finish(true)
	assert.False(t, g.challenge("colleague", "correct", "192.0.2.2"))
	assert.True(t, g.challenge("next", "unknown", "192.0.2.2"))
	assert.False(t, g.challenge("next", "unknown", "192.0.2.3"))
}

func TestLoginGuardDistributedAndConcurrentReservations(t *testing.T) {
	g := newLoginGuard()
	for i := range 10 {
		finish, err := g.begin("target", "wrong", fmt.Sprintf("192.0.2.%d", i+1), true, false)
		require.NoError(t, err)
		defer finish(false)
	}
	_, err := g.begin("target", "guess", "192.0.2.99", true, false)
	assert.ErrorIs(t, err, ErrLoginChallenge)
}

func TestLoginGuardBasicLimitsAndKnownCredentials(t *testing.T) {
	g := newLoginGuard()
	finish, err := g.begin("target", "correct", "192.0.2.1", false, false)
	require.NoError(t, err)
	finish(true)
	for range 10 {
		finish, err = g.begin(" TARGET ", "wrong", "192.0.2.1:1000", false, false)
		require.NoError(t, err)
		finish(false)
	}
	_, err = g.begin("target", "wrong", "192.0.2.1:2000", false, false)
	var limit *LoginRateLimitError
	require.ErrorAs(t, err, &limit)
	assert.Positive(t, limit.RetryAfter)
	finish, err = g.begin("colleague", "correct", "192.0.2.1:2000", false, false)
	require.NoError(t, err)
	finish(true)
	finish, err = g.begin("target", "correct", "192.0.2.1:2000", false, false)
	require.NoError(t, err)
	finish(false)
	// A proof is never authentication, and is removed if the password was changed/revoked.
	assert.False(t, g.now().Before(g.proofs[g.proof("target", "correct")]))
}

func TestLoginGuardNormalGitTrafficDoesNotConsumeSharedBudgets(t *testing.T) {
	g := newLoginGuard()
	finish, err := g.begin("owner", "correct", "192.0.2.1", false, false)
	require.NoError(t, err)
	finish(true)
	for range 300 {
		finish, err = g.begin("owner", "correct", "192.0.2.1", false, false)
		require.NoError(t, err)
		finish(true)
	}
	assert.False(t, g.challenge("colleague", "new", "192.0.2.1"))
	for range 5 {
		finish, err = g.begin("owner", "wrong", "192.0.2.1", true, false)
		require.NoError(t, err)
		finish(false)
	}
	finish, err = g.begin("owner", "correct", "192.0.2.1", false, false)
	require.NoError(t, err)
	finish(true)
	assert.True(t, g.challenge("owner", "different-guess", "192.0.2.1"))
}

func TestLoginGuardExpiryAndCapacity(t *testing.T) {
	g := newLoginGuard()
	now := time.Now()
	g.now = func() time.Time { return now }
	for range 5 {
		finish, err := g.begin("target", "wrong", "192.0.2.1", true, false)
		require.NoError(t, err)
		finish(false)
	}
	assert.True(t, g.challenge("target", "wrong", "192.0.2.1"))
	now = now.Add(16 * time.Minute)
	assert.False(t, g.challenge("target", "wrong", "192.0.2.1"))
	assert.Empty(t, g.buckets)
	for i := range loginMaxEntries {
		g.buckets[loginKey(strconv.Itoa(i))] = &loginBucket{window: now, expires: now.Add(time.Hour)}
	}
	assert.True(t, g.challenge("new", "wrong", "192.0.2.1"))
	_, err := g.begin("new", "wrong", "192.0.2.1", false, false)
	assert.Error(t, err)
	finish, err := g.begin("new", "correct", "192.0.2.1", true, true)
	require.NoError(t, err)
	finish(true)
	assert.LessOrEqual(t, len(g.buckets), loginMaxEntries)
}

func TestLoginGuardConcurrencyBound(t *testing.T) {
	g := newLoginGuard()
	var wg sync.WaitGroup
	for i := range loginMaxConcurrent {
		finish, err := g.begin(strconv.Itoa(i), "wrong", "192.0.2.1", false, false)
		require.NoError(t, err)
		defer finish(false)
	}
	_, err := g.begin("overflow", "wrong", "192.0.2.2", true, true)
	assert.Error(t, err)
	// Concurrent read paths must not race with attempt accounting.
	for i := range 100 {
		wg.Go(func() { g.challenge(strconv.Itoa(i), "wrong", "192.0.2.1") })
	}
	wg.Wait()
}

func TestLoginGuardRequestAddressAndDisabled(t *testing.T) {
	for input, want := range map[string]string{"192.0.2.1:22": "192.0.2.1", "[2001:db8::1]:123": "2001:db8::1", "[::ffff:192.0.2.1]:3": "192.0.2.1"} {
		assert.Equal(t, want, normalizeRemoteAddr(input))
	}
	defer test.MockVariableValue(&setting.LoginProtectionEnabled, true)()
	req := httptest.NewRequest(http.MethodPost, "/user/login", nil)
	req.RemoteAddr = "192.0.2.10:10"
	req.Header.Set("X-Forwarded-For", "192.0.2.20")
	ctx := WithLoginRequest(context.Background(), req)
	assert.Equal(t, "192.0.2.10:10", ctx.Value(loginRequestKey{}).(*http.Request).RemoteAddr)
	defer test.MockVariableValue(&setting.LoginProtectionEnabled, false)()
	assert.False(t, LoginChallengeRequired(ctx, "user", "password"))
	assert.Equal(t, 429, LoginErrorStatus(&LoginRateLimitError{RetryAfter: 1}))
}

func TestLoginGuardSecondFactor(t *testing.T) {
	defer test.MockVariableValue(&passwordLoginGuard, newLoginGuard())()
	defer test.MockVariableValue(&setting.LoginProtectionEnabled, true)()
	for range 10 {
		require.NoError(t, AllowSecondFactor(1))
	}
	require.Error(t, AllowSecondFactor(1))
	require.NoError(t, AllowSecondFactor(2))
}
