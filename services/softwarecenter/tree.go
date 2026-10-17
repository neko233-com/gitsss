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
		for parent := s.Nodes[id].Parent; parent != ""; parent = s.Nodes[parent].Parent {
			if chosen[parent] {
				covered = true
				break
			}
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
	top, err := selection(s, ids)
	if err != nil {
		return err
	}
	removed := map[string]bool{}
	for _, id := range top {
		for child := range descendants(s, id) {
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
	if err = save(ctx, orgID, s); err != nil {
		return err
	}
	reclaim(orgID, s)
	if err = save(ctx, orgID, s); err != nil {
		log.Warn("Software center cleanup receipt pending for organization %d: %v", orgID, err)
	}
	return nil
}

func Move(ctx context.Context, orgID int64, ids []string, target string) error {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return err
	}
	if !s.folder(target) {
		return ErrInvalid
	}
	top, err := selection(s, ids)
	if err != nil {
		return err
	}
	names := []string{}
	for _, id := range top {
		node := s.Nodes[id]
		subtree := descendants(s, id)
		for _, upload := range s.Uploads {
			if subtree[upload.Parent] {
				return ErrBusy
			}
		}
		for parent := target; parent != ""; parent = s.Nodes[parent].Parent {
			if parent == id {
				return ErrInvalid
			}
		}
		for _, name := range names {
			if strings.EqualFold(name, node.Name) {
				return ErrConflict
			}
		}
		names = append(names, node.Name)
		if err = s.checkName(target, node.Name, id); err != nil {
			return err
		}
		parentDepth := 0
		for p := target; p != ""; p = s.Nodes[p].Parent {
			parentDepth++
		}
		for child := range subtree {
			depth := parentDepth + 1
			for p := child; p != id; p = s.Nodes[p].Parent {
				depth++
			}
			if depth > 32 {
				return ErrInvalid
			}
		}
	}
	for _, id := range top {
		s.Nodes[id].Parent = target
		s.Nodes[id].Modified = time.Now().Unix()
	}
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
	return result, save(ctx, orgID, s)
}
