// Copyright 2026 gitsss Authors. All rights reserved.

package audit

import (
	"sort"
	"sync"
	"time"
)

type Entry struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	RequestID string    `json:"request_id"`
	Result    string    `json:"result,omitempty"`
	At        time.Time `json:"at"`
}

type Log struct {
	mu      sync.RWMutex
	entries []Entry
}

func (l *Log) Append(entry Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *Log) Entries() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := append([]Entry(nil), l.entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].At.After(entries[j].At) })
	return entries
}

// Cleanup applies an explicit audit retention policy; it is deliberately not an LRU eviction.
func (l *Log) Cleanup(before time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.entries[:0]
	removed := 0
	for _, entry := range l.entries {
		if entry.At.Before(before) {
			removed++
			continue
		}
		kept = append(kept, entry)
	}
	l.entries = kept
	return removed
}
