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

const MaxLinks = 32

var (
	ErrInvalid  = errors.New("invalid homepage links")
	ErrConflict = errors.New("homepage links changed in another session")
	storeLock   sync.Mutex
)

type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type State struct {
	Links   []Link `json:"links"`
	ownerID int64
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
	storeLock.Lock()
	defer storeLock.Unlock()
	state, err := Load(ctx, ownerID)
	if err != nil {
		return err
	}
	if state.Revision() != revision {
		return ErrConflict
	}
	state.Links = validated
	return system.AppState.Set(ctx, state)
}
