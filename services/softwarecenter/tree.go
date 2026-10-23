// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/neko233-com/gitsss/modules/log"
)

const MaxSelection = 500

func selection(s *State, ids []string) ([]string, error) {
	if len(ids) == 0 || len(ids) > MaxSelection {
		return nil, ErrInvalid
	}
	chosen := map[string]bool{}
	for _, id := range ids {
		if s.Nodes[id] == nil {
			return nil, ErrInvalid
		}
		chosen[id] = true
	}
	top := []string{}
	for id := range chosen {
		covered := false
		for depth, parent := 0, s.Nodes[id].Parent; parent != ""; depth++ {
			if depth > 64 || s.Nodes[parent] == nil {
				return nil, ErrInvalid
			}
			if chosen[parent] {
				covered = true
				break
			}
			parent = s.Nodes[parent].Parent
		}
		if !covered {
			top = append(top, id)
		}
	}
	return top, nil
}

func DeleteMany(ctx context.Context, orgID int64, ids []string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	prune(orgID, s)
	top, err := selection(s, ids)
	if err != nil {
		return err
	}
	children := childIndex(s)
	removed := map[string]bool{}
	for _, id := range top {
		for child := range descendantsFrom(children, id) {
			removed[child] = true
		}
	}
	for _, r := range s.Uploads {
		if removed[r.Parent] {
			return ErrBusy
		}
	}
	digests := map[string]bool{}
	for id := range removed {
		if digest := s.Nodes[id].Digest; digest != "" {
			digests[digest] = true
		}
		delete(s.Nodes, id)
	}
	for _, node := range s.Nodes {
		delete(digests, node.Digest)
	}
	for digest := range digests {
		s.Garbage[digest] = time.Now().Unix()
	}
	s.Generation++
	if err = save(ctx, orgID, s); err != nil {
		return err
	}
	reclaim(orgID, s)
	if err = save(ctx, orgID, s); err != nil {
		log.Warn("Software center cleanup receipt pending for organization %d: %v", orgID, err)
	}
	return nil
}

func subtreeHeight(index map[string][]string, id string) int {
	type level struct {
		id    string
		depth int
	}
	height := 0
	visited := map[string]bool{}
	stack := []level{{id, 0}}
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[item.id] {
			continue
		}
		visited[item.id] = true
		height = max(height, item.depth)
		for _, child := range index[item.id] {
			stack = append(stack, level{child, item.depth + 1})
		}
	}
	return height
}

func Move(ctx context.Context, orgID int64, ids []string, target string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	prune(orgID, s)
	if !s.folder(target) {
		return ErrInvalid
	}
	top, err := selection(s, ids)
	if err != nil {
		return err
	}
	children := childIndex(s)
	parentDepth := 0
	for parent := target; parent != ""; {
		node := s.Nodes[parent]
		if node == nil || parentDepth >= 32 {
			return ErrInvalid
		}
		parentDepth++
		parent = node.Parent
	}
	taken := map[string]int{}
	for _, node := range s.Nodes {
		if node.Parent == target {
			taken[foldName(node.Name)]++
		}
	}
	for _, upload := range s.Uploads {
		if upload.Parent == target {
			taken[foldName(upload.Name)]++
		}
	}
	moving := map[string]bool{}
	for _, id := range top {
		nodes := descendantsFrom(children, id)
		for node := range nodes {
			moving[node] = true
		}
		for depth, parent := 0, target; parent != ""; depth++ {
			if parent == id || depth > 64 {
				return ErrInvalid
			}
			parent = s.Nodes[parent].Parent
		}
		if parentDepth+1+subtreeHeight(children, id) > 32 {
			return ErrInvalid
		}
	}
	for _, upload := range s.Uploads {
		if moving[upload.Parent] {
			return ErrBusy
		}
	}
	now := time.Now().Unix()
	for _, id := range top {
		node := s.Nodes[id]
		if node.Parent == target {
			continue
		}
		key := foldName(node.Name)
		if taken[key] > 0 {
			return ErrConflict
		}
		taken[key]++
		node.Parent = target
		node.Modified = now
	}
	s.Generation++
	return save(ctx, orgID, s)
}

func foldName(name string) string {
	return strings.Map(func(r rune) rune {
		lowest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < lowest {
				lowest = next
			}
		}
		return lowest
	}, name)
}

// EnsureDirectories creates one bounded batch atomically, retaining the uploaded relative paths.
func EnsureDirectories(ctx context.Context, orgID, userID int64, parent string, paths []string) (map[string]string, error) {
	if len(paths) > MaxSelection {
		return nil, ErrInvalid
	}
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	prune(orgID, s)
	if !s.folder(parent) {
		return nil, ErrInvalid
	}
	index := map[string]map[string]*Node{}
	for _, node := range s.Nodes {
		if index[node.Parent] == nil {
			index[node.Parent] = map[string]*Node{}
		}
		index[node.Parent][foldName(node.Name)] = node
	}
	pending := map[string]bool{}
	for _, upload := range s.Uploads {
		pending[upload.Parent+":"+foldName(upload.Name)] = true
	}
	parentDepth := 0
	for current := parent; current != ""; current = s.Nodes[current].Parent {
		parentDepth++
	}
	result := map[string]string{"": parent}
	created := false
	for _, path := range paths {
		parts := strings.Split(path, "/")
		if len(parts)+parentDepth > 32 {
			return nil, ErrInvalid
		}
		current := parent
		for i, name := range parts {
			if !validName(name) {
				return nil, ErrInvalid
			}
			node := index[current][foldName(name)]
			if node != nil {
				if !node.Directory || node.Name != name {
					return nil, ErrConflict
				}
			} else {
				if len(s.Nodes)+len(s.Uploads) >= MaxEntries {
					return nil, ErrQuota
				}
				if pending[current+":"+foldName(name)] {
					return nil, ErrConflict
				}
				node = &Node{ID: newID(), Parent: current, Name: name, Directory: true, Modified: time.Now().Unix(), UserID: userID}
				s.Nodes[node.ID] = node
				created = true
				if index[current] == nil {
					index[current] = map[string]*Node{}
				}
				index[current][foldName(name)] = node
			}
			current = node.ID
			result[strings.Join(parts[:i+1], "/")] = current
		}
		result[path] = current
	}
	if created {
		s.Generation++
	}
	return result, save(ctx, orgID, s)
}
