// Copyright 2026 gitsss Authors. All rights reserved.

package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	schemaVersion    = 1
	maxDocumentBytes = 1 << 20
)

var ErrInvalidDocument = errors.New("invalid state document")

type Document struct {
	ID      string
	Payload json.RawMessage
}

type Store struct {
	database *sql.DB
}

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%w: database path is required", ErrInvalidDocument)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(absolute); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: database must be a regular file", ErrInvalidDocument)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return nil, createErr
		}
		if closeErr := file.Close(); closeErr != nil {
			return nil, closeErr
		}
	} else {
		return nil, err
	}
	if err := os.Chmod(absolute, 0o600); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", sqliteDSN(absolute))
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	store := &Store{database: database}
	if err := store.initialize(); err != nil {
		database.Close()
		return nil, err
	}
	if err := secureSQLiteSidecars(absolute); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) Close() error {
	if store == nil || store.database == nil {
		return nil
	}
	return store.database.Close()
}

func (store *Store) Put(namespace, id string, payload json.RawMessage) error {
	if !validKey(namespace, 64) || !validKey(id, 256) || len(payload) == 0 || len(payload) > maxDocumentBytes || !json.Valid(payload) {
		return ErrInvalidDocument
	}
	_, err := store.database.Exec(`
		INSERT INTO state_documents (namespace, id, payload)
		VALUES (?, ?, ?)
		ON CONFLICT(namespace, id) DO UPDATE SET payload = excluded.payload
	`, namespace, id, []byte(payload))
	return err
}

func (store *Store) List(namespace string) ([]Document, error) {
	if !validKey(namespace, 64) {
		return nil, ErrInvalidDocument
	}
	rows, err := store.database.Query(`
		SELECT id, payload FROM state_documents
		WHERE namespace = ? ORDER BY id
	`, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := make([]Document, 0)
	for rows.Next() {
		var document Document
		if err := rows.Scan(&document.ID, &document.Payload); err != nil {
			return nil, err
		}
		if len(document.Payload) == 0 || len(document.Payload) > maxDocumentBytes || !json.Valid(document.Payload) {
			return nil, fmt.Errorf("%w: corrupt payload for %s", ErrInvalidDocument, document.ID)
		}
		document.Payload = append(json.RawMessage(nil), document.Payload...)
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func (store *Store) initialize() error {
	var journalMode string
	if err := store.database.QueryRow("PRAGMA journal_mode = WAL").Scan(&journalMode); err != nil {
		return err
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("embedded state database did not enable WAL mode")
	}
	settings := []string{
		"PRAGMA synchronous = FULL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, setting := range settings {
		if _, err := store.database.Exec(setting); err != nil {
			return err
		}
	}
	transaction, err := store.database.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.Exec(`
		CREATE TABLE IF NOT EXISTS state_documents (
			namespace TEXT NOT NULL,
			id TEXT NOT NULL,
			payload BLOB NOT NULL,
			PRIMARY KEY (namespace, id)
		) WITHOUT ROWID
	`); err != nil {
		return err
	}
	var version int
	if err := transaction.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("state database schema %d is newer than supported version %d", version, schemaVersion)
	}
	if version < schemaVersion {
		if _, err := transaction.Exec("PRAGMA user_version = 1"); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func secureSQLiteSidecars(path string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecar := path + suffix
		info, err := os.Lstat(sidecar)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: SQLite sidecar must be a regular file", ErrInvalidDocument)
		}
		if err := os.Chmod(sidecar, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func sqliteDSN(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	query := make(url.Values)
	query.Set("mode", "rwc")
	return "file:" + strings.Join(parts, "/") + "?" + query.Encode()
}

func validKey(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}
