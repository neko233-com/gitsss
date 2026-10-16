// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package homelinks

import (
	"context"
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
