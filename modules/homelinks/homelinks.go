// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package homelinks stores shared and personal dashboard shortcuts in app state.
package homelinks

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/system"
)

const (
	MaxLinks    = 10000
	PageSize    = 24
	MaxPageRows = 64
)

var (
	ErrInvalid  = errors.New("invalid homepage links")
	ErrConflict = errors.New("homepage links changed in another session")
	storeLocks  [64]sync.Mutex
)

type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type State struct {
	Links          []Link `json:"links"`
	ownerID        int64
	revision       int64
	exists         bool
	loadedRevision string
}

// Name keeps personal records isolated without changing storage paths or schema.
func (s *State) Name() string {
	if s.ownerID == 0 {
		return "enterprise-home-links"
	}
	return PersonalKey(s.ownerID)
}

func PersonalKey(ownerID int64) string {
	return "personal-home-links-" + strconv.FormatInt(ownerID, 10)
}

func (s *State) Revision() string {
	data, _ := json.Marshal(s.Links)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// LoadedRevision is valid until the caller mutates Links.
func (s *State) LoadedRevision() string {
	if s.loadedRevision != "" {
		return s.loadedRevision
	}
	return s.Revision()
}

func Validate(links []Link) ([]Link, error) {
	if len(links) > MaxLinks {
		return nil, ErrInvalid
	}
	out := make([]Link, 0, len(links))
	for _, link := range links {
		link.Title, link.URL = strings.TrimSpace(link.Title), strings.TrimSpace(link.URL)
		if link.Title == "" || utf8.RuneCountInString(link.Title) > 120 || len(link.URL) > 2048 || strings.ContainsAny(link.URL, "\\") || strings.ContainsFunc(link.URL, unicode.IsSpace) || strings.ContainsFunc(link.URL, unicode.IsControl) {
			return nil, ErrInvalid
		}
		u, err := url.Parse(link.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
			return nil, ErrInvalid
		}
		out = append(out, link)
	}
	return out, nil
}

func Load(ctx context.Context, ownerID int64) (*State, error) {
	if system.AppState == nil || ownerID < 0 {
		return nil, errors.New("homepage link store unavailable")
	}
	state := &State{ownerID: ownerID, Links: []Link{}}
	if store, ok := system.AppState.(*system.DBStore); ok {
		if cached := cachedState(ownerID); cached != nil {
			revision, exists, err := store.Revision(ctx, state.Name())
			if err != nil {
				return nil, err
			}
			if cached.revision == revision && cached.exists == exists {
				return cached, nil
			}
		}
		var err error
		state.revision, state.exists, err = store.GetVersioned(ctx, state)
		if err != nil {
			return nil, err
		}
		state.loadedRevision = state.Revision()
		rememberState(state)
		return state, nil
	}
	if err := system.AppState.Get(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

// Save rejects stale editors instead of silently overwriting their changes.
func Save(ctx context.Context, ownerID int64, revision string, links []Link) error {
	validated, err := Validate(links)
	if err != nil {
		return err
	}
	lock := &storeLocks[uint64(ownerID)%uint64(len(storeLocks))]
	lock.Lock()
	defer lock.Unlock()
	state, err := Load(ctx, ownerID)
	if err != nil {
		return err
	}
	if state.LoadedRevision() != revision {
		return ErrConflict
	}
	state.Links = validated
	return saveState(ctx, state)
}

// IndexedLink refers to a position in one unchanged scope revision; -1 adds an item.
type IndexedLink struct {
	Index int
	Link
}

// ApplyPage updates the visible positions and preserves all other entries.
func ApplyPage(ctx context.Context, ownerID int64, revision string, original []int, submitted []IndexedLink) error {
	if ownerID < 0 || len(original) > PageSize || len(submitted) > MaxPageRows {
		return ErrInvalid
	}
	links := make([]Link, len(submitted))
	for i := range submitted {
		links[i] = submitted[i].Link
	}
	valid, err := Validate(links)
	if err != nil {
		return err
	}
	lock := &storeLocks[uint64(ownerID)%uint64(len(storeLocks))]
	lock.Lock()
	defer lock.Unlock()
	state, err := Load(ctx, ownerID)
	if err != nil {
		return err
	}
	if state.LoadedRevision() != revision {
		return ErrConflict
	}
	visible := make(map[int]bool, len(original))
	for _, index := range original {
		if index < 0 || index >= len(state.Links) || visible[index] {
			return ErrInvalid
		}
		visible[index] = true
	}
	replacements := make(map[int]Link, len(original))
	added := make([]Link, 0, len(submitted))
	for i, item := range submitted {
		if item.Index == -1 {
			added = append(added, valid[i])
			continue
		}
		if !visible[item.Index] {
			return ErrInvalid
		}
		if _, exists := replacements[item.Index]; exists {
			return ErrInvalid
		}
		replacements[item.Index] = valid[i]
	}
	if len(state.Links)-len(original)+len(replacements)+len(added) > MaxLinks {
		return ErrInvalid
	}
	result := state.Links[:0]
	for index, link := range state.Links {
		if visible[index] {
			if replacement, exists := replacements[index]; exists {
				result = append(result, replacement)
			}
		} else {
			result = append(result, link)
		}
	}
	result = append(result, added...)
	state.Links = result
	return saveState(ctx, state)
}

func saveState(ctx context.Context, state *State) error {
	if store, ok := system.AppState.(*system.DBStore); ok {
		changed, err := store.CompareAndSet(ctx, state, state.revision, state.exists)
		forgetState(state.ownerID)
		if err != nil {
			return err
		}
		if !changed {
			return ErrConflict
		}
		return nil
	}
	return system.AppState.Set(ctx, state)
}
