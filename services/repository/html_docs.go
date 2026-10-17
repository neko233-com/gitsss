// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/models/system"
	"github.com/neko233-com/gitsss/modules/json"
)

var ErrHTMLDocsConflict = errors.New("HTML Docs settings changed; reload before saving")

// HTMLDocsSettings selects a Git-backed site without creating another copy of its files.
type HTMLDocsSettings struct {
	Enabled   bool   `json:"enabled"`
	Ref       string `json:"ref,omitempty"`
	Directory string `json:"directory,omitempty"`
	Entry     string `json:"entry,omitempty"`
	Revision  int64  `json:"-"`
}

func htmlDocsKey(repoID int64) string { return fmt.Sprintf("repo-html-docs-%d", repoID) }

// CleanHTMLDocsPath rejects traversal rather than resolving it outside the configured site.
func CleanHTMLDocsPath(value string) (string, error) {
	if len(value) > 2048 || strings.ContainsAny(value, "\\\x00\r\n?#") || strings.HasPrefix(value, "/") {
		return "", errors.New("invalid relative documentation path")
	}
	for segment := range strings.SplitSeq(value, "/") {
		if segment == ".." || strings.HasPrefix(segment, ".git") {
			return "", errors.New("documentation path cannot contain parent or Git metadata directories")
		}
	}
	return strings.TrimPrefix(path.Clean("./"+value), "./"), nil
}

func (s *HTMLDocsSettings) Validate() error {
	s.Ref = strings.TrimSpace(s.Ref)
	if len(s.Ref) > 255 || strings.ContainsAny(s.Ref, "\x00\r\n") || strings.HasPrefix(s.Ref, "-") {
		return errors.New("invalid documentation branch, tag or commit")
	}
	var err error
	s.Directory, err = CleanHTMLDocsPath(strings.TrimSpace(s.Directory))
	if err != nil {
		return err
	}
	if s.Directory == "." {
		s.Directory = ""
	}
	if s.Entry == "" {
		s.Entry = "index.html"
	}
	s.Entry, err = CleanHTMLDocsPath(s.Entry)
	if err != nil {
		return err
	}
	if !strings.EqualFold(path.Ext(s.Entry), ".html") && !strings.EqualFold(path.Ext(s.Entry), ".htm") {
		return errors.New("documentation entry must be an HTML file")
	}
	return nil
}

func GetHTMLDocsSettings(ctx context.Context, repoID int64) (*HTMLDocsSettings, error) {
	state := &system.AppState{ID: htmlDocsKey(repoID)}
	has, err := db.GetEngine(ctx).ID(state.ID).Get(state)
	if err != nil {
		return nil, err
	}
	settings := &HTMLDocsSettings{Enabled: true, Entry: "index.html"}
	if has {
		if err := json.Unmarshal([]byte(state.Content), settings); err != nil {
			return nil, err
		}
		settings.Revision = state.Revision
	}
	return settings, settings.Validate()
}

func SaveHTMLDocsSettings(ctx context.Context, repoID int64, settings *HTMLDocsSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	content, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		engine := db.GetEngine(ctx)
		key := htmlDocsKey(repoID)
		result, err := engine.Exec("UPDATE app_state SET content=?, revision=revision+1 WHERE id=? AND revision=?", string(content), key, settings.Revision)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows > 0 {
			return err
		}
		exists, err := engine.ID(key).Exist(&system.AppState{})
		if err != nil {
			return err
		}
		if exists || settings.Revision != 0 {
			return ErrHTMLDocsConflict
		}
		_, err = engine.Insert(&system.AppState{ID: key, Revision: 1, Content: string(content)})
		return err
	})
}
