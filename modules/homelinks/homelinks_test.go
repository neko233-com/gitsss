// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package homelinks

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/system"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	links, err := Validate([]Link{{Title: " CI/CD ", URL: " http://192.168.1.2:8080/path?q=1 "}, {Title: "DB", URL: "https://db.example/"}})
	require.NoError(t, err)
	assert.Equal(t, "CI/CD", links[0].Title)
	for _, invalid := range []Link{
		{Title: "", URL: "https://example.com"},
		{Title: strings.Repeat("文", 121), URL: "https://example.com"},
		{Title: "x", URL: "javascript:alert(1)"},
		{Title: "x", URL: "//example.com"},
		{Title: "x", URL: "https://user:password@example.com"},
		{Title: "x", URL: "file:///secret"},
		{Title: "x", URL: "https:///missing"},
		{Title: "x", URL: "https://example.com/\npath"},
		{Title: "x", URL: "https://example.com\\@evil.example"},
	} {
		_, err := Validate([]Link{invalid})
		assert.ErrorIs(t, err, ErrInvalid, "%+v", invalid)
	}
	_, err = Validate(make([]Link, MaxLinks+1))
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestLargeCollectionAndPageUpdates(t *testing.T) {
	previous := system.AppState
	system.AppState = memoryStore{}
	t.Cleanup(func() { system.AppState = previous })
	ctx := t.Context()
	state, err := Load(ctx, 5)
	require.NoError(t, err)
	links := make([]Link, MaxLinks)
	for i := range links {
		links[i] = Link{Title: fmt.Sprintf("Link %05d", i), URL: fmt.Sprintf("https://host.internal/%d", i)}
	}
	require.NoError(t, Save(ctx, 5, state.Revision(), links))
	state, err = Load(ctx, 5)
	require.NoError(t, err)
	first := FindPage(nil, state.Links, "", 1)
	assert.Len(t, first.Entries, PageSize)
	assert.Equal(t, MaxLinks, first.Total)
	last := FindPage(nil, state.Links, "", 9999)
	assert.Equal(t, 417, last.Number)
	assert.Len(t, last.Entries, 16)
	assert.Equal(t, links[9999], last.Entries[15].Link)
	match := FindPage(nil, state.Links, "LINK 09999", 2)
	assert.Equal(t, 1, match.Number)
	assert.Equal(t, 1, match.Total)
	assert.Equal(t, 9999, match.Entries[0].Index)
	match = FindPage(nil, state.Links, "HOST.INTERNAL/9999", 1)
	assert.Equal(t, 1, match.Total)
	combined := FindPage(links[:23], links[23:26], "", 1)
	assert.Len(t, combined.Shared, 23)
	assert.Len(t, combined.Personal, 1)

	revision := state.Revision()
	update := []IndexedLink{{10, Link{Title: "Changed", URL: "https://changed.internal"}}, {-1, Link{Title: "Added", URL: "https://added.internal"}}}
	assert.ErrorIs(t, ApplyPage(ctx, 5, revision, []int{10, 10}, update), ErrInvalid)
	assert.ErrorIs(t, ApplyPage(ctx, 5, revision, []int{11}, update), ErrInvalid)
	assert.ErrorIs(t, ApplyPage(ctx, 5, revision, nil, []IndexedLink{{Index: -1, Link: links[0]}}), ErrInvalid)
	require.NoError(t, ApplyPage(ctx, 5, revision, []int{10, 12}, update))
	assert.ErrorIs(t, ApplyPage(ctx, 5, revision, nil, nil), ErrConflict)
	updated, err := Load(ctx, 5)
	require.NoError(t, err)
	assert.Len(t, updated.Links, MaxLinks)
	assert.Equal(t, links[9], updated.Links[9])
	assert.Equal(t, "Changed", updated.Links[10].Title)
	assert.Equal(t, links[13], updated.Links[12])
	assert.Equal(t, links[9999], updated.Links[9998])
	assert.Equal(t, "Added", updated.Links[9999].Title)
	other, err := Load(ctx, 6)
	require.NoError(t, err)
	assert.Empty(t, other.Links)
}

type memoryStore map[string][]byte

func (s memoryStore) Get(_ context.Context, item system.StateItem) error {
	if b := s[item.Name()]; b != nil {
		return json.Unmarshal(b, item)
	}
	return nil
}

func (s memoryStore) Set(_ context.Context, item system.StateItem) (err error) {
	s[item.Name()], err = json.Marshal(item)
	return err
}

func TestIsolationAndStaleSave(t *testing.T) {
	previous := system.AppState
	system.AppState = memoryStore{}
	t.Cleanup(func() { system.AppState = previous })
	for _, owner := range []int64{0, 5, 6} {
		state, err := Load(t.Context(), owner)
		require.NoError(t, err)
		require.Empty(t, state.Links)
		revision := state.Revision()
		require.NoError(t, Save(t.Context(), owner, revision, []Link{{Title: state.Name(), URL: "http://ci.internal:8080"}}))
		assert.ErrorIs(t, Save(t.Context(), owner, revision, nil), ErrConflict)
		saved, err := Load(t.Context(), owner)
		require.NoError(t, err)
		assert.Equal(t, saved.Name(), saved.Links[0].Title)
		require.NoError(t, Save(t.Context(), owner, saved.Revision(), nil))
	}
}
