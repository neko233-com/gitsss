// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package announcement stores site-wide notices shown after sign-in.
package announcement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/system"
)

// Notice is one enterprise announcement.
type Notice struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Published   bool   `json:"published"`
	CreatedUnix int64  `json:"created_unix"`
}

// State is the persisted announcement list.
type State struct {
	Notices []Notice `json:"notices"`
}

// Name is the app-state key.
func (s *State) Name() string {
	return "enterprise-announcements"
}

// Published returns notices that signed-in users should see, newest first.
func Published(notices []Notice) []Notice {
	out := make([]Notice, 0, len(notices))
	for _, notice := range slices.Backward(notices) {
		if notice.Published && strings.TrimSpace(notice.Title) != "" {
			out = append(out, notice)
		}
	}
	return out
}

// Upsert validates and inserts or replaces a notice.
func Upsert(notices []Notice, notice Notice, now time.Time) ([]Notice, error) {
	notice.Title = strings.TrimSpace(notice.Title)
	notice.Body = strings.TrimSpace(notice.Body)
	if notice.Title == "" {
		return nil, errors.New("announcement title is required")
	}
	if len(notice.Title) > 200 || len(notice.Body) > 20000 {
		return nil, errors.New("announcement is too long")
	}
	if notice.ID == "" {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		notice.ID = id
		notice.CreatedUnix = now.Unix()
		return append(notices, notice), nil
	}
	for i := range notices {
		if notices[i].ID == notice.ID {
			notice.CreatedUnix = notices[i].CreatedUnix
			notices[i] = notice
			return notices, nil
		}
	}
	return nil, errors.New("announcement was not found")
}

// Delete removes one notice by id.
func Delete(notices []Notice, id string) []Notice {
	out := make([]Notice, 0, len(notices))
	for _, notice := range notices {
		if notice.ID != id {
			out = append(out, notice)
		}
	}
	return out
}

// Load reads announcements. A missing record is an empty list.
func Load(ctx context.Context) ([]Notice, error) {
	if system.AppState == nil {
		return nil, errors.New("announcement store is not ready")
	}
	state := &State{}
	if err := system.AppState.Get(ctx, state); err != nil {
		return nil, err
	}
	return state.Notices, nil
}

// Save writes the full list.
func Save(ctx context.Context, notices []Notice) error {
	if system.AppState == nil {
		return errors.New("announcement store is not ready")
	}
	return system.AppState.Set(ctx, &State{Notices: notices})
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
