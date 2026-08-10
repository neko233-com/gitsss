// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testLoginFailureCache struct {
	values map[string]string
}

func (c *testLoginFailureCache) Get(key string) (string, bool) {
	value, ok := c.values[key]
	return value, ok
}

func (c *testLoginFailureCache) Put(key, value string, _ int64) error {
	c.values[key] = value
	return nil
}

func (c *testLoginFailureCache) Delete(key string) error {
	delete(c.values, key)
	return nil
}

func TestLoginFailureThrottleDoesNotLockSuccessfulLogin(t *testing.T) {
	c := &testLoginFailureCache{values: map[string]string{}}
	var delay int64
	for range 5 {
		delay = int64(RecordLoginFailure(context.Background(), c, "owner", "10.0.0.2"))
	}
	assert.Positive(t, delay)

	ClearLoginFailures(context.Background(), c, "owner", "10.0.0.2")
	assert.Empty(t, c.values)
	assert.Zero(t, RecordLoginFailure(context.Background(), c, "owner", "10.0.0.2"))
}

func TestLoginFailureThrottleNormalizesRemotePort(t *testing.T) {
	c := &testLoginFailureCache{values: map[string]string{}}
	for range 4 {
		RecordLoginFailure(context.Background(), c, "owner", "10.0.0.2:1000")
	}

	assert.Positive(t, RecordLoginFailure(context.Background(), c, "other", "10.0.0.2:2000"))
	ClearLoginFailures(context.Background(), c, "other", "10.0.0.2:3000")
	assert.Positive(t, RecordLoginFailure(context.Background(), c, "owner", "10.0.0.2:4000"))
}
