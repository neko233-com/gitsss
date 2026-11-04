// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"testing"

	"github.com/neko233-com/gitsss/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{FixtureFiles: []string{ /* load nothing */ }})
}

type testVersionedItem struct{ Value string }

func (*testVersionedItem) Name() string { return "test-versioned-item" }

func TestAppStateCompareAndSet(t *testing.T) {
	store := &DBStore{}
	item := &testVersionedItem{Value: "first"}
	revision, exists, err := store.GetVersioned(t.Context(), item)
	require.NoError(t, err)
	require.False(t, exists)
	changed, err := store.CompareAndSet(t.Context(), item, revision, exists)
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = store.CompareAndSet(t.Context(), item, revision, exists)
	require.NoError(t, err)
	assert.False(t, changed)
	revision, exists, err = store.Revision(t.Context(), item.Name())
	require.NoError(t, err)
	require.True(t, exists)
	assert.EqualValues(t, 1, revision)
	item.Value = "second"
	changed, err = store.CompareAndSet(t.Context(), item, revision, exists)
	require.NoError(t, err)
	assert.True(t, changed)
	changed, err = store.CompareAndSet(t.Context(), item, revision, exists)
	require.NoError(t, err)
	assert.False(t, changed)
	loaded := &testVersionedItem{}
	next, exists, err := store.GetVersioned(t.Context(), loaded)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.EqualValues(t, 2, next)
	assert.Equal(t, "second", loaded.Value)
}

type testItem1 struct {
	Val1 string
	Val2 int
}

func (*testItem1) Name() string {
	return "test-item1"
}

type testItem2 struct {
	K string
}

func (*testItem2) Name() string {
	return "test-item2"
}

func TestAppStateDB(t *testing.T) {
	as := &DBStore{}

	item1 := new(testItem1)
	assert.NoError(t, as.Get(t.Context(), item1))
	assert.Empty(t, item1.Val1)
	assert.Equal(t, 0, item1.Val2)

	item1 = new(testItem1)
	item1.Val1 = "a"
	item1.Val2 = 2
	assert.NoError(t, as.Set(t.Context(), item1))

	item2 := new(testItem2)
	item2.K = "V"
	assert.NoError(t, as.Set(t.Context(), item2))

	item1 = new(testItem1)
	assert.NoError(t, as.Get(t.Context(), item1))
	assert.Equal(t, "a", item1.Val1)
	assert.Equal(t, 2, item1.Val2)

	item2 = new(testItem2)
	assert.NoError(t, as.Get(t.Context(), item2))
	assert.Equal(t, "V", item2.K)
}
