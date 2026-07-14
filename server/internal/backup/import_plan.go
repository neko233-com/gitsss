// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ImportStatus string

const (
	ImportNew       ImportStatus = "new"
	ImportIdentical ImportStatus = "identical"
	ImportConflict  ImportStatus = "conflict"
	ImportBlocked   ImportStatus = "blocked"
)

type ImportItem struct {
	Path        string       `json:"path"`
	Destination string       `json:"destination"`
	Kind        string       `json:"kind"`
	Size        int64        `json:"size"`
	Status      ImportStatus `json:"status"`
	Reason      string       `json:"reason,omitempty"`
}

type ImportSummary struct {
	New       int `json:"new"`
	Identical int `json:"identical"`
	Conflicts int `json:"conflicts"`
	Blocked   int `json:"blocked"`
}

type ImportPlan struct {
	AssessmentID string        `json:"assessment_id"`
	Target       string        `json:"target"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Summary      ImportSummary `json:"summary"`
	Items        []ImportItem  `json:"items"`
}

func PreviewImport(inputPath, passphrase, target string) (ImportPlan, error) {
	verification, err := Verify(inputPath, passphrase)
	if err != nil {
		return ImportPlan{}, err
	}
	return BuildImportPlan(verification, target, time.Now())
}

func BuildImportPlan(verification Verification, target string, now time.Time) (ImportPlan, error) {
	if strings.TrimSpace(target) == "" {
		return ImportPlan{}, fmt.Errorf("%w: import target is required", ErrInvalidBackup)
	}
	if err := validateManifest(verification.Manifest); err != nil {
		return ImportPlan{}, err
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return ImportPlan{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	plan := ImportPlan{
		AssessmentID: verification.Manifest.Source.AssessmentID,
		Target:       absolute,
		GeneratedAt:  now.UTC(),
		Items:        make([]ImportItem, 0, len(verification.Manifest.Entries)),
	}
	rootBlockReason := inspectImportAncestors(absolute)
	for _, entry := range verification.Manifest.Entries {
		item := ImportItem{}
		if rootBlockReason != "" {
			item = ImportItem{
				Path: entry.Path, Destination: filepath.Join(absolute, filepath.FromSlash(entry.Path)),
				Kind: entry.Kind, Size: entry.Size, Status: ImportBlocked, Reason: rootBlockReason,
			}
		} else {
			item = inspectImportTarget(absolute, entry)
		}
		plan.Items = append(plan.Items, item)
		switch item.Status {
		case ImportNew:
			plan.Summary.New++
		case ImportIdentical:
			plan.Summary.Identical++
		case ImportConflict:
			plan.Summary.Conflicts++
		case ImportBlocked:
			plan.Summary.Blocked++
		}
	}
	return plan, nil
}

func inspectImportAncestors(root string) string {
	cursor := root
	for {
		info, err := os.Lstat(cursor)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "target path contains a symlink"
			}
			if !info.IsDir() {
				return "target path contains a non-directory ancestor"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "target path ancestor cannot be inspected"
		}
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return ""
		}
		cursor = parent
	}
}

func inspectImportTarget(root string, entry Entry) ImportItem {
	destination := filepath.Join(root, filepath.FromSlash(entry.Path))
	item := ImportItem{Path: entry.Path, Destination: destination, Kind: entry.Kind, Size: entry.Size}
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		item.Status = ImportBlocked
		item.Reason = "destination escapes the import target"
		return item
	}
	rootInfo, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		item.Status = ImportNew
		return item
	}
	if err != nil {
		item.Status = ImportBlocked
		item.Reason = "target root cannot be inspected"
		return item
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		item.Status = ImportBlocked
		item.Reason = "target root is a symlink"
		return item
	}
	if !rootInfo.IsDir() {
		item.Status = ImportBlocked
		item.Reason = "target root is not a directory"
		return item
	}

	components := strings.Split(relative, string(filepath.Separator))
	cursor := root
	for index, component := range components {
		cursor = filepath.Join(cursor, component)
		info, err := os.Lstat(cursor)
		if errors.Is(err, os.ErrNotExist) {
			item.Status = ImportNew
			return item
		}
		if err != nil {
			item.Status = ImportBlocked
			item.Reason = "destination cannot be inspected"
			return item
		}
		if info.Mode()&os.ModeSymlink != 0 {
			item.Status = ImportBlocked
			item.Reason = "destination path contains a symlink"
			return item
		}
		if index < len(components)-1 {
			if !info.IsDir() {
				item.Status = ImportBlocked
				item.Reason = "destination parent is not a directory"
				return item
			}
			continue
		}
		if !info.Mode().IsRegular() {
			item.Status = ImportConflict
			item.Reason = "destination is not a regular file"
			return item
		}
		if info.Size() != entry.Size {
			item.Status = ImportConflict
			item.Reason = "file size differs"
			return item
		}
		_, digest, err := hashFile(cursor)
		if err != nil {
			item.Status = ImportBlocked
			item.Reason = "destination cannot be hashed"
			return item
		}
		if digest != entry.SHA256 {
			item.Status = ImportConflict
			item.Reason = "file checksum differs"
			return item
		}
		item.Status = ImportIdentical
		return item
	}
	item.Status = ImportBlocked
	item.Reason = "destination could not be resolved"
	return item
}
