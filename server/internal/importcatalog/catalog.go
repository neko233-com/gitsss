// Copyright 2026 gitsss Authors. All rights reserved.

package importcatalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

const maxBodyBytes = 64 << 10

var ErrInvalidImport = errors.New("invalid imported business record")

type DirectoryEntry struct {
	SourceID     string    `json:"source_id"`
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Organization bool      `json:"organization"`
	Active       bool      `json:"active"`
	Visibility   int64     `json:"visibility,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

type WorkItem struct {
	SourceID    string    `json:"source_id"`
	Repository  string    `json:"repository_id"`
	Index       int64     `json:"index"`
	Title       string    `json:"title"`
	Body        string    `json:"body,omitempty"`
	BodyCut     bool      `json:"body_truncated"`
	Author      string    `json:"author_id,omitempty"`
	PullRequest bool      `json:"pull_request"`
	Closed      bool      `json:"closed"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type AccessEntry struct {
	SourceID   string `json:"source_id"`
	Repository string `json:"repository_id"`
	User       string `json:"user_id"`
	Mode       int64  `json:"mode"`
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Catalog struct {
	store *state.Store
}

type sourceRow struct {
	SourceTable string         `json:"source_table"`
	Ordinal     int64          `json:"ordinal"`
	Values      map[string]any `json:"values"`
	External    string         `json:"external,omitempty"`
}

func New(store *state.Store) (*Catalog, error) {
	if store == nil {
		return nil, ErrInvalidImport
	}
	return &Catalog{store: store}, nil
}

func (catalog *Catalog) Directory(after string, limit int) (Page[DirectoryEntry], error) {
	documents, err := catalog.store.ListPage("source-import-user", after, normalizedLimit(limit))
	if err != nil {
		return Page[DirectoryEntry]{}, err
	}
	page := Page[DirectoryEntry]{Items: make([]DirectoryEntry, 0, len(documents))}
	for _, document := range documents {
		row, err := decodeRow(document, "user")
		if err != nil {
			return Page[DirectoryEntry]{}, err
		}
		name := firstText(row.Values, "name", "lower_name", "login_name")
		if name == "" {
			return Page[DirectoryEntry]{}, fmt.Errorf("%w: user %s has no name", ErrInvalidImport, document.ID)
		}
		active := true
		if value, exists := row.Values["is_active"]; exists {
			active = boolean(value)
		}
		page.Items = append(page.Items, DirectoryEntry{
			SourceID: stringValue(row.Values["id"]), Name: name,
			DisplayName:  firstText(row.Values, "full_name", "name"),
			Organization: integer(row.Values["type"]) == 1, Active: active,
			Visibility: integer(row.Values["visibility"]), CreatedAt: timestamp(row.Values, "created_unix", "created"),
		})
	}
	page.NextCursor = nextCursor(documents, limit)
	return page, nil
}

func (catalog *Catalog) WorkItems(repositoryID, after string, limit int) (Page[WorkItem], error) {
	prefix, err := sourceIDPrefix(repositoryID)
	if err != nil {
		return Page[WorkItem]{}, err
	}
	documents, err := catalog.store.ListPrefix("import-work-items", prefix, after, normalizedLimit(limit))
	if err != nil {
		return Page[WorkItem]{}, err
	}
	page := Page[WorkItem]{Items: make([]WorkItem, 0, len(documents))}
	for _, document := range documents {
		row, err := decodeRow(document, "issue")
		if err != nil {
			return Page[WorkItem]{}, err
		}
		body, truncated := boundedText(firstText(row.Values, "content", "body"))
		page.Items = append(page.Items, WorkItem{
			SourceID: stringValue(row.Values["id"]), Repository: stringValue(row.Values["repo_id"]),
			Index: integer(row.Values["index"]), Title: firstText(row.Values, "name", "title"),
			Body: body, BodyCut: truncated || boolean(row.Values["_body_truncated"]), Author: stringValue(row.Values["poster_id"]),
			PullRequest: boolean(row.Values["is_pull"]), Closed: boolean(row.Values["is_closed"]),
			CreatedAt: timestamp(row.Values, "created_unix", "created"), UpdatedAt: timestamp(row.Values, "updated_unix", "updated"),
		})
	}
	page.NextCursor = nextCursor(documents, limit)
	return page, nil
}

func sourceIDPrefix(value string) (string, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return "", ErrInvalidImport
	}
	return fmt.Sprintf("%020d-", parsed), nil
}

func (catalog *Catalog) Access(after string, limit int) (Page[AccessEntry], error) {
	documents, err := catalog.store.ListPage("source-import-access", after, normalizedLimit(limit))
	if err != nil {
		return Page[AccessEntry]{}, err
	}
	page := Page[AccessEntry]{Items: make([]AccessEntry, 0, len(documents))}
	for _, document := range documents {
		row, err := decodeRow(document, "access")
		if err != nil {
			return Page[AccessEntry]{}, err
		}
		page.Items = append(page.Items, AccessEntry{
			SourceID: document.ID, Repository: stringValue(row.Values["repo_id"]),
			User: stringValue(row.Values["user_id"]), Mode: integer(row.Values["mode"]),
		})
	}
	page.NextCursor = nextCursor(documents, limit)
	return page, nil
}

func decodeRow(document state.Document, table string) (sourceRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(document.Payload))
	decoder.UseNumber()
	var row sourceRow
	if err := decoder.Decode(&row); err != nil || row.SourceTable != table || row.Ordinal < 1 || row.External != "" || row.Values == nil {
		return sourceRow{}, fmt.Errorf("%w: corrupt %s row %s", ErrInvalidImport, table, document.ID)
	}
	return row, nil
}

func normalizedLimit(limit int) int {
	if limit < 1 {
		return 100
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func nextCursor(documents []state.Document, requested int) string {
	limit := normalizedLimit(requested)
	if len(documents) == limit {
		return documents[len(documents)-1].ID
	}
	return ""
}

func firstText(values map[string]any, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(stringValue(values[name])); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func integer(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case bool:
		if typed {
			return 1
		}
	}
	return 0
}

func boolean(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		if parsed {
			return true
		}
	}
	return integer(value) != 0
}

func timestamp(values map[string]any, names ...string) time.Time {
	for _, name := range names {
		value := values[name]
		seconds := integer(value)
		if seconds > 0 {
			return time.Unix(seconds, 0).UTC()
		}
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
				return parsed.UTC()
			}
		}
	}
	return time.Time{}
}

func boundedText(value string) (string, bool) {
	if len(value) <= maxBodyBytes {
		return value, false
	}
	return value[:maxBodyBytes], true
}
