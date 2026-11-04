// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package homelinks

import (
	"container/list"
	"sync"
)

const cacheBudget = 32 << 20

type cacheEntry struct {
	ownerID int64
	state   *State
	size    int64
}

var linkCache = struct {
	sync.Mutex
	entries map[int64]*list.Element
	lru     list.List
	bytes   int64
}{entries: make(map[int64]*list.Element)}

func cloneState(state *State) *State {
	copyState := *state
	copyState.Links = append([]Link{}, state.Links...)
	return &copyState
}

func cachedState(ownerID int64) *State {
	linkCache.Lock()
	defer linkCache.Unlock()
	if element := linkCache.entries[ownerID]; element != nil {
		linkCache.lru.MoveToFront(element)
		return cloneState(element.Value.(*cacheEntry).state)
	}
	return nil
}

func forgetState(ownerID int64) {
	linkCache.Lock()
	defer linkCache.Unlock()
	if element := linkCache.entries[ownerID]; element != nil {
		linkCache.bytes -= element.Value.(*cacheEntry).size
		linkCache.lru.Remove(element)
		delete(linkCache.entries, ownerID)
	}
}

func rememberState(state *State) {
	size := int64(256)
	for _, link := range state.Links {
		size += int64(len(link.Title) + len(link.URL) + 32)
	}
	if size > cacheBudget {
		forgetState(state.ownerID)
		return
	}
	linkCache.Lock()
	defer linkCache.Unlock()
	if element := linkCache.entries[state.ownerID]; element != nil {
		linkCache.bytes -= element.Value.(*cacheEntry).size
		linkCache.lru.Remove(element)
		delete(linkCache.entries, state.ownerID)
	}
	for linkCache.bytes+size > cacheBudget {
		oldest := linkCache.lru.Back()
		entry := oldest.Value.(*cacheEntry)
		linkCache.bytes -= entry.size
		delete(linkCache.entries, entry.ownerID)
		linkCache.lru.Remove(oldest)
	}
	entry := &cacheEntry{ownerID: state.ownerID, state: cloneState(state), size: size}
	linkCache.entries[state.ownerID] = linkCache.lru.PushFront(entry)
	linkCache.bytes += size
}
