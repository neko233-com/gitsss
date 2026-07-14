// Copyright 2026 gitsss Authors. All rights reserved.

package audit

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

const stateNamespace = "audit-entries"

var (
	ErrInvalidEntry = errors.New("invalid audit entry")
	ErrState        = errors.New("audit state unavailable")
)

type Entry struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	RequestID string    `json:"request_id"`
	Result    string    `json:"result,omitempty"`
	At        time.Time `json:"at"`
}

type record struct {
	id    string
	entry Entry
}

type Log struct {
	mu      sync.RWMutex
	records []record
	store   *state.Store
	vault   *state.Vault
}

func NewPersistentLog(store *state.Store, vault *state.Vault) (*Log, error) {
	if store == nil || vault == nil {
		return nil, errors.New("audit state store and vault are required")
	}
	documents, err := store.List(stateNamespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	log := &Log{records: make([]record, 0, len(documents)), store: store, vault: vault}
	for _, document := range documents {
		plaintext, err := vault.Open(stateNamespace, document.ID, document.Payload)
		if err != nil {
			return nil, fmt.Errorf("%w: encrypted entry %q could not be opened", ErrState, document.ID)
		}
		var entry Entry
		if err := json.Unmarshal(plaintext, &entry); err != nil || !validID(document.ID) || !validEntry(entry) {
			return nil, fmt.Errorf("%w: corrupt entry %q", ErrState, document.ID)
		}
		log.records = append(log.records, record{id: document.ID, entry: entry})
	}
	return log, nil
}

func (l *Log) Append(entry Entry) error {
	entry.At = entry.At.UTC()
	if !validEntry(entry) {
		return ErrInvalidEntry
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	record := record{id: newID(), entry: entry}
	if l.store != nil {
		plaintext, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrState, err)
		}
		envelope, err := l.vault.Seal(stateNamespace, record.id, plaintext)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrState, err)
		}
		if err := l.store.Put(stateNamespace, record.id, envelope); err != nil {
			return fmt.Errorf("%w: %v", ErrState, err)
		}
	}
	l.records = append(l.records, record)
	return nil
}

func (l *Log) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := make([]Entry, 0, len(l.records))
	for _, record := range l.records {
		entries = append(entries, record.entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].At.After(entries[j].At) })
	return entries
}

// Cleanup applies an explicit audit retention policy; it is deliberately not an LRU eviction.
func (l *Log) Cleanup(before time.Time) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	kept := make([]record, 0, len(l.records))
	removed := make([]string, 0)
	for _, record := range l.records {
		if record.entry.At.Before(before) {
			removed = append(removed, record.id)
			continue
		}
		kept = append(kept, record)
	}
	if l.store != nil {
		if err := l.store.DeleteMany(stateNamespace, removed); err != nil {
			return 0, fmt.Errorf("%w: %v", ErrState, err)
		}
	}
	l.records = kept
	return len(removed), nil
}

func validEntry(entry Entry) bool {
	return validText(entry.Actor, 128, true) && validText(entry.Action, 128, true) &&
		validText(entry.Target, 4096, true) && validText(entry.RequestID, 128, false) &&
		validText(entry.Result, 256, false) && !entry.At.IsZero()
}

func validText(value string, maximum int, required bool) bool {
	if required && strings.TrimSpace(value) == "" {
		return false
	}
	return len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}

func validID(id string) bool {
	decoded, err := hex.DecodeString(strings.TrimPrefix(id, "audit-"))
	return strings.HasPrefix(id, "audit-") && err == nil && len(decoded) == 12
}

func newID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("audit-%024x", uint64(time.Now().UnixNano()))
	}
	return "audit-" + hex.EncodeToString(value)
}
