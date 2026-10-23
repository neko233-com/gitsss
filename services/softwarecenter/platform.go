// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/json"
)

// Platform is inferred from the suffix, never from a directory or an architecture name.
func Platform(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".exe", ".msi", ".msix", ".msixbundle", ".appx", ".appxbundle":
		return "Windows"
	case ".dmg", ".pkg", ".app":
		return "macOS"
	case ".ipa":
		return "iOS"
	case ".apk", ".aab", ".apks", ".xapk":
		return "Android"
	case ".zip", ".7z", ".rar", ".tar", ".gz", ".tgz", ".bz2", ".tbz2", ".xz", ".txz", ".zst":
		return "all"
	default:
		return "unknown"
	}
}

func platformFolder(platform string) string {
	if platform == "all" {
		return "全平台"
	}
	for _, name := range []string{"Android", "iOS", "macOS", "Windows"} {
		if platform == name {
			return name
		}
	}
	return ""
}

type Entry struct {
	*SoftwareNodeInfo
	Platform string `json:"platform"`
	Path     string `json:"path"`
}

type Listing struct {
	Entries     []*Entry            `json:"entries"`
	Folders     []*SoftwareNodeInfo `json:"folders"`
	Breadcrumbs []*SoftwareNodeInfo `json:"breadcrumbs"`
	Used        int64               `json:"used"`
	Reserved    int64               `json:"reserved"`
	Quota       int64               `json:"quota"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
}

func nodePath(s *State, id, parent string) string {
	parts := []string{}
	for depth := 0; id != parent && id != ""; depth++ {
		node := s.Nodes[id]
		if node == nil || depth > 64 {
			return ""
		}
		parts = append(parts, node.Name)
		id = node.Parent
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

func nodeLess(a, b *Node) bool {
	if a.Directory != b.Directory {
		return a.Directory
	}
	if aName, bName := strings.ToLower(a.Name), strings.ToLower(b.Name); aName != bName {
		return aName < bName
	}
	return a.ID < b.ID
}

func infoLess(a, b *SoftwareNodeInfo) bool {
	if a.Directory != b.Directory {
		return a.Directory
	}
	if aName, bName := strings.ToLower(a.Name), strings.ToLower(b.Name); aName != bName {
		return aName < bName
	}
	return a.ID < b.ID
}

func List(ctx context.Context, orgID int64, parent, query string, page int, recursive bool) (*Listing, error) {
	return ListFiltered(ctx, orgID, parent, query, page, recursive, "")
}

func normalizePlatformFilter(platform string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "":
		return "", true
	case "windows":
		return "Windows", true
	case "macos":
		return "macOS", true
	case "ios":
		return "iOS", true
	case "android":
		return "Android", true
	case "all":
		return "all", true
	case "unknown":
		return "unknown", true
	default:
		return "", false
	}
}

func ListFiltered(ctx context.Context, orgID int64, parent, query string, page int, recursive bool, platform string) (*Listing, error) {
	s, err := Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !s.folder(parent) {
		return nil, ErrInvalid
	}
	platform, ok := normalizePlatformFilter(platform)
	if !ok {
		return nil, ErrInvalid
	}
	result := &Listing{Entries: []*Entry{}, Folders: []*SoftwareNodeInfo{}, Breadcrumbs: []*SoftwareNodeInfo{}, Used: s.Used(), Reserved: s.Reserved(), Quota: s.Quota}
	query = strings.ToLower(strings.TrimSpace(query))
	subtree := map[string]bool{}
	if recursive {
		subtree = descendants(s, parent)
	}
	matched := []*Node{}
	for _, node := range s.Nodes {
		if node.Directory {
			result.Folders = append(result.Folders, publicNode(node))
		}
		matches := node.Parent == parent
		if recursive {
			matches = node.ID != parent && subtree[node.ID]
		} else if query != "" {
			matches = true // Preserve the web page's global search contract.
		}
		if matches && (query == "" || strings.Contains(strings.ToLower(node.Name), query)) && (node.Directory || platform == "" || strings.EqualFold(Platform(node.Name), platform)) {
			matched = append(matched, node)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return nodeLess(matched[i], matched[j]) })
	sort.Slice(result.Folders, func(i, j int) bool { return infoLess(result.Folders[i], result.Folders[j]) })
	for depth, id := 0, parent; id != ""; depth++ {
		node := s.Nodes[id]
		if node == nil || depth > 64 {
			return nil, ErrInvalid
		}
		result.Breadcrumbs = append(result.Breadcrumbs, publicNode(node))
		id = node.Parent
	}
	result.Total = len(matched)
	result.Page = min(max(page, 1), max((result.Total+199)/200, 1))
	start := min((result.Page-1)*200, result.Total)
	for _, node := range matched[start:min(start+200, result.Total)] {
		platform := ""
		if !node.Directory {
			platform = Platform(node.Name)
		}
		result.Entries = append(result.Entries, &Entry{SoftwareNodeInfo: publicNode(node), Platform: platform, Path: nodePath(s, node.ID, "")})
	}
	return result, nil
}

// OrganizeOptions selects a subtree and optionally applies a previously previewed revision.
// swagger:model SoftwareOrganizeOptions
type OrganizeOptions struct {
	Parent    string            `json:"parent"`
	Apply     bool              `json:"apply"`
	Revision  string            `json:"revision"`
	Platforms map[string]string `json:"platforms"`
}

type PlannedMove struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Platform string `json:"platform"`
	Reason   string `json:"reason,omitempty"`
}

type OrganizeReport struct {
	Applied  bool          `json:"applied"`
	Revision string        `json:"revision"`
	Moves    []PlannedMove `json:"moves"`
	Skipped  []PlannedMove `json:"skipped"`
	Created  []string      `json:"created"`
}

// Organize changes only directory metadata and parent IDs in one atomic save.
func Organize(ctx context.Context, orgID, userID int64, opts OrganizeOptions) (*OrganizeReport, error) {
	guard := lock(orgID)
	guard.Lock()
	defer guard.Unlock()
	s, err := Load(ctx, orgID)
	if err != nil {
		return nil, err
	}
	prune(orgID, s)
	if !s.folder(opts.Parent) || len(opts.Platforms) > MaxSelection {
		return nil, ErrInvalid
	}
	nodeIDs, uploadIDs := []string{}, []string{}
	for id := range s.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	for id := range s.Uploads {
		uploadIDs = append(uploadIDs, id)
	}
	slices.Sort(nodeIDs)
	slices.Sort(uploadIDs)
	// Progress fields change during transfers and must not expire a previewed plan.
	state := struct {
		Parent    string            `json:"parent"`
		Platforms map[string]string `json:"platforms,omitempty"`
		Nodes     []any             `json:"nodes"`
		Uploads   []any             `json:"uploads"`
	}{Parent: opts.Parent, Platforms: opts.Platforms}
	for _, id := range nodeIDs {
		node := s.Nodes[id]
		state.Nodes = append(state.Nodes, id, node.Parent, node.Name, node.Directory)
	}
	for _, id := range uploadIDs {
		upload := s.Uploads[id]
		state.Uploads = append(state.Uploads, id, upload.Parent, upload.Name, upload.Size)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	revision := fmt.Sprintf("%x", sha256.Sum256(raw))
	if opts.Apply && (opts.Revision == "" || opts.Revision != revision) {
		return nil, ErrConflict
	}
	subtree := descendants(s, opts.Parent)
	for id, platform := range opts.Platforms {
		if s.Nodes[id] == nil || s.Nodes[id].Directory || !subtree[id] || platformFolder(platform) == "" {
			return nil, ErrInvalid
		}
	}
	report := &OrganizeReport{Revision: revision, Moves: []PlannedMove{}, Skipped: []PlannedMove{}, Created: []string{}}
	files := []*Node{}
	originalPaths := map[string]string{}
	for _, node := range s.Nodes {
		originalPaths[node.ID] = nodePath(s, node.ID, opts.Parent)
		if !node.Directory && subtree[node.ID] {
			files = append(files, node)
		}
	}
	sort.Slice(files, func(i, j int) bool { return nodeLess(files[i], files[j]) })
	index := map[string]*Node{}
	for _, node := range s.Nodes {
		index[node.Parent+":"+foldName(node.Name)] = node
	}
	pending := map[string]bool{}
	for _, upload := range s.Uploads {
		pending[upload.Parent+":"+foldName(upload.Name)] = true
	}
	checkName := func(parent, name, except string) error {
		key := parent + ":" + foldName(name)
		if existing := index[key]; (existing != nil && existing.ID != except) || pending[key] {
			return ErrConflict
		}
		depth := 0
		for id := parent; id != ""; id = s.Nodes[id].Parent {
			depth++
			if depth >= 32 {
				return ErrInvalid
			}
		}
		return nil
	}
	ensure := func(parent, name, relative string) (string, error) {
		key := parent + ":" + foldName(name)
		if node := index[key]; node != nil {
			if !node.Directory {
				return "", ErrConflict
			}
			return node.ID, nil
		}
		if len(s.Nodes)+len(s.Uploads) >= MaxEntries {
			return "", ErrQuota
		}
		if err := checkName(parent, name, ""); err != nil {
			return "", err
		}
		node := &Node{ID: newID(), Parent: parent, Name: name, Directory: true, Modified: time.Now().Unix(), UserID: userID}
		s.Nodes[node.ID], index[key] = node, node
		report.Created = append(report.Created, relative)
		return node.ID, nil
	}
	// Reuse purpose folders so splitting platforms does not leave empty categories at the root.
	categories := []*Node{}
	for _, node := range s.Nodes {
		if node.Directory && node.Parent == opts.Parent {
			isPlatform := false
			for _, platform := range []string{"Android", "iOS", "macOS", "Windows", "all"} {
				isPlatform = isPlatform || strings.EqualFold(node.Name, platformFolder(platform))
			}
			if !isPlatform {
				categories = append(categories, node)
			}
		}
	}
	sort.Slice(categories, func(i, j int) bool { return nodeLess(categories[i], categories[j]) })
	categoryOf := func(id string) string {
		for id != "" && s.Nodes[id].Parent != opts.Parent {
			id = s.Nodes[id].Parent
		}
		return id
	}
	countsByCategory := map[string]map[string]int{}
	blockedCategories := map[string]bool{}
	for _, upload := range s.Uploads {
		blockedCategories[categoryOf(upload.Parent)] = true
	}
	for _, file := range files {
		id := categoryOf(file.ID)
		platform := Platform(file.Name)
		if override, ok := opts.Platforms[file.ID]; ok {
			platform = override
		}
		if countsByCategory[id] == nil {
			countsByCategory[id] = map[string]int{}
		}
		countsByCategory[id][platform]++
		blockedCategories[id] = blockedCategories[id] || platform == "unknown"
	}
	for _, category := range categories {
		if blockedCategories[category.ID] {
			continue
		}
		counts := countsByCategory[category.ID]
		platform := "all"
		for _, candidate := range []string{"Windows", "macOS", "iOS", "Android", "all"} {
			if counts[candidate] > counts[platform] {
				platform = candidate
			}
		}
		target, err := ensure(opts.Parent, platformFolder(platform), platformFolder(platform))
		if err != nil {
			return nil, err
		}
		if index[target+":"+foldName(category.Name)] != nil {
			continue // Keep both folders when a destination already exists; never merge by deletion.
		}
		if err := checkName(target, category.Name, category.ID); err != nil {
			return nil, err
		}
		delete(index, category.Parent+":"+foldName(category.Name))
		category.Parent, category.Modified = target, time.Now().Unix()
		index[target+":"+foldName(category.Name)] = category
		report.Moves = append(report.Moves, PlannedMove{ID: category.ID, From: originalPaths[category.ID], To: platformFolder(platform) + "/" + category.Name, Platform: platform})
	}
	for _, node := range files {
		platform := Platform(node.Name)
		if override, ok := opts.Platforms[node.ID]; ok {
			platform = override
		}
		from := originalPaths[node.ID]
		move := PlannedMove{ID: node.ID, From: from, Platform: platform}
		folder := platformFolder(platform)
		if folder == "" {
			move.Reason = "unknown_suffix"
			report.Skipped = append(report.Skipped, move)
			continue
		}
		parts := strings.Split(from, "/")
		if len(parts) > 1 {
			for _, platform := range []string{"Android", "iOS", "macOS", "Windows", "all"} {
				if strings.EqualFold(parts[0], platformFolder(platform)) {
					parts = parts[1:]
					break
				}
			}
		}
		move.To = folder + "/" + strings.Join(parts, "/")
		if from == move.To {
			continue
		}
		target := opts.Parent
		directories := append([]string{folder}, parts[:len(parts)-1]...)
		for i, name := range directories {
			target, err = ensure(target, name, strings.Join(directories[:i+1], "/"))
			if err != nil {
				return nil, err
			}
		}
		if node.Parent == target {
			continue
		}
		if err = checkName(target, node.Name, node.ID); err != nil {
			return nil, err
		}
		delete(index, node.Parent+":"+foldName(node.Name))
		node.Parent = target
		node.Modified = time.Now().Unix()
		index[target+":"+foldName(node.Name)] = node
		report.Moves = append(report.Moves, move)
	}
	for _, node := range s.Nodes {
		depth := 0
		for id := node.ID; id != ""; id = s.Nodes[id].Parent {
			depth++
			if depth > 32 {
				return nil, ErrInvalid
			}
		}
	}
	if opts.Apply && (len(report.Created) > 0 || len(report.Moves) > 0) {
		s.Generation++
		if err := save(ctx, orgID, s); err != nil {
			return nil, err
		}
	}
	report.Applied = opts.Apply
	return report, nil
}
