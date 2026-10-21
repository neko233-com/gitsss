// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package receiveflow overlaps validation while preserving native Git ref transactions.
package receiveflow

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maximumHeaderBytes = 4 * 1024 * 1024

type update struct {
	old, next, ref string
}

type key struct {
	repository, ref, oid string
}

type pending struct {
	ready chan struct{}
	users atomic.Int64
	done  atomic.Bool
}

func (entry *pending) signal() {
	if entry.done.CompareAndSwap(false, true) {
		close(entry.ready)
	}
}

type Registry struct {
	entries sync.Map
}

var active Registry

func validOID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}

func readCommands(input io.Reader) (io.Reader, []update) {
	reader := bufio.NewReader(input)
	var prefix bytes.Buffer
	var updates []update
	for prefix.Len() <= maximumHeaderBytes {
		header := make([]byte, 4)
		count, err := io.ReadFull(reader, header)
		prefix.Write(header[:count])
		if err != nil {
			break
		}
		length, err := strconv.ParseUint(string(header), 16, 16)
		if err != nil || length < 4 {
			break
		}
		data := make([]byte, int(length)-4)
		count, err = io.ReadFull(reader, data)
		prefix.Write(data[:count])
		if err != nil {
			break
		}
		command, _, _ := strings.Cut(string(data), "\x00")
		fields := strings.Fields(command)
		if len(fields) != 3 || !validOID(fields[0]) || !validOID(fields[1]) || !strings.HasPrefix(fields[2], "refs/") {
			// Signed or unknown protocol extensions remain wholly native Git's concern.
			updates = nil
			break
		}
		old, next := strings.ToLower(fields[0]), strings.ToLower(fields[1])
		if old != next && strings.Trim(next, "0") != "" {
			updates = append(updates, update{old: old, next: next, ref: fields[2]})
		}
	}
	if prefix.Len() > maximumHeaderBytes {
		updates = nil
	}
	return io.MultiReader(bytes.NewReader(prefix.Bytes()), reader), updates
}

func (registry *Registry) claim(id key) *pending {
	candidate := &pending{ready: make(chan struct{})}
	candidate.users.Store(1)
	for {
		value, loaded := registry.entries.LoadOrStore(id, candidate)
		if !loaded {
			return candidate
		}
		entry := value.(*pending)
		for users := entry.users.Load(); users >= 0; users = entry.users.Load() {
			if entry.users.CompareAndSwap(users, users+1) {
				return entry
			}
		}
		registry.entries.CompareAndDelete(id, entry)
	}
}

// Register preserves every input byte; registrations live only until this RPC exits.
func (registry *Registry) Register(repository string, input io.Reader) (io.Reader, func()) {
	reader, updates := readCommands(input)
	claims := make(map[key]*pending, len(updates))
	for _, command := range updates {
		id := key{repository: repository, ref: command.ref, oid: command.next}
		if _, exists := claims[id]; !exists {
			claims[id] = registry.claim(id)
		}
	}
	return reader, sync.OnceFunc(func() {
		for id, entry := range claims {
			if entry.users.Add(-1) == 0 && entry.users.CompareAndSwap(0, -1) {
				entry.signal()
				registry.entries.CompareAndDelete(id, entry)
			}
		}
	})
}

// Wait runs only after authorization and validation, never instead of Git's CAS.
func (registry *Registry) Wait(ctx context.Context, repository string, oldIDs, newIDs, refs []string) {
	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	for index, old := range oldIDs {
		if index >= len(newIDs) || index >= len(refs) || strings.EqualFold(old, newIDs[index]) || strings.Trim(old, "0") == "" {
			continue
		}
		value, exists := registry.entries.Load(key{repository: repository, ref: refs[index], oid: strings.ToLower(old)})
		if !exists {
			continue
		}
		select {
		case <-value.(*pending).ready:
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		}
	}
}

// Committed is called by the existing post-receive API after Git updated the ref.
func (registry *Registry) Committed(repository string, newIDs, refs []string) {
	for index, oid := range newIDs {
		if index >= len(refs) {
			continue
		}
		if value, exists := registry.entries.Load(key{repository: repository, ref: refs[index], oid: strings.ToLower(oid)}); exists {
			value.(*pending).signal()
		}
	}
}

func Register(repository string, input io.Reader) (io.Reader, func()) {
	return active.Register(repository, input)
}

func Wait(ctx context.Context, repository string, oldIDs, newIDs, refs []string) {
	active.Wait(ctx, repository, oldIDs, newIDs, refs)
}

func Committed(repository string, newIDs, refs []string) {
	active.Committed(repository, newIDs, refs)
}
