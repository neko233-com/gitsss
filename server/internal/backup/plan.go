// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/takeover"
)

const maxManifestEntries = 1_000_000

var (
	ErrInvalidBackup = errors.New("invalid gitsss backup")
	ErrSourceChanged = errors.New("backup source changed after assessment")
)

type Source struct {
	AssessmentID string `json:"assessment_id"`
	ConfigDigest string `json:"config_digest"`
	GiteaVersion string `json:"gitea_version,omitempty"`
}

type Entry struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Modified int64  `json:"modified_unix"`
}

type Manifest struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Source    Source    `json:"source"`
	Entries   []Entry   `json:"entries"`
}

type sourceEntry struct {
	path  string
	entry Entry
}

type Plan struct {
	Manifest Manifest
	sources  []sourceEntry
}

func BuildPlan(report takeover.Report, now time.Time) (Plan, error) {
	if strings.TrimSpace(report.ID) == "" || strings.TrimSpace(report.ConfigDigest) == "" {
		return Plan{}, fmt.Errorf("%w: assessment identity is required", ErrInvalidBackup)
	}
	if now.IsZero() {
		now = time.Now()
	}
	plan := Plan{Manifest: Manifest{
		Version:   1,
		CreatedAt: now.UTC(),
		Source: Source{
			AssessmentID: report.ID,
			ConfigDigest: report.ConfigDigest,
			GiteaVersion: report.GiteaVersion,
		},
	}}
	seen := map[string]struct{}{}
	if err := plan.addFile("config/app.ini", "config", report.ConfigPath, seen); err != nil {
		return Plan{}, fmt.Errorf("add configuration: %w", err)
	}
	for _, asset := range report.Assets {
		if !asset.Exists {
			continue
		}
		if !asset.Readable || asset.Truncated {
			return Plan{}, fmt.Errorf("%w: asset %q was not fully assessed", ErrInvalidBackup, asset.Kind)
		}
		if err := plan.addAsset(asset, seen); err != nil {
			return Plan{}, err
		}
	}
	if len(plan.sources) > maxManifestEntries {
		return Plan{}, fmt.Errorf("%w: manifest exceeds %d entries", ErrInvalidBackup, maxManifestEntries)
	}
	sort.Slice(plan.sources, func(left, right int) bool {
		return plan.sources[left].entry.Path < plan.sources[right].entry.Path
	})
	plan.Manifest.Entries = make([]Entry, len(plan.sources))
	for index := range plan.sources {
		plan.Manifest.Entries[index] = plan.sources[index].entry
	}
	return plan, nil
}

func (plan *Plan) addAsset(asset takeover.Asset, seen map[string]struct{}) error {
	kind := strings.TrimSpace(asset.Kind)
	if !safeKind(kind) {
		return fmt.Errorf("%w: unsafe asset kind %q", ErrInvalidBackup, kind)
	}
	root := filepath.Clean(asset.Path)
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", kind, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: symlink asset root %s", ErrInvalidBackup, root)
	}
	if info.Mode().IsRegular() {
		return plan.addFile(path.Join("data", kind, filepath.Base(root)), kind, root, seen)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: asset %s is not a regular file or directory", ErrInvalidBackup, kind)
	}
	return filepath.WalkDir(root, func(filePath string, directory os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if directory.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink found in %s", ErrInvalidBackup, kind)
		}
		if directory.IsDir() {
			return nil
		}
		info, err := directory.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: non-regular file found in %s", ErrInvalidBackup, kind)
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		return plan.addFile(path.Join("data", kind, filepath.ToSlash(relative)), kind, filePath, seen)
	})
}

func (plan *Plan) addFile(archivePath, kind, sourcePath string, seen map[string]struct{}) error {
	if !safeArchivePath(archivePath) {
		return fmt.Errorf("%w: unsafe archive path %q", ErrInvalidBackup, archivePath)
	}
	if _, exists := seen[archivePath]; exists {
		return fmt.Errorf("%w: duplicate archive path %q", ErrInvalidBackup, archivePath)
	}
	info, digest, err := hashFile(sourcePath)
	if err != nil {
		return err
	}
	seen[archivePath] = struct{}{}
	plan.sources = append(plan.sources, sourceEntry{path: sourcePath, entry: Entry{
		Path: archivePath, Kind: kind, Size: info.Size(), SHA256: digest,
		Modified: info.ModTime().Unix(),
	}})
	return nil
}

func hashFile(filePath string) (os.FileInfo, string, error) {
	linkInfo, err := os.Lstat(filePath)
	if err != nil {
		return nil, "", err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("%w: %s is a symlink", ErrInvalidBackup, filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%w: %s is not a regular file", ErrInvalidBackup, filePath)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, "", err
	}
	return info, hex.EncodeToString(hasher.Sum(nil)), nil
}

func safeKind(kind string) bool {
	if kind == "" || len(kind) > 64 {
		return false
	}
	for _, character := range kind {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
			continue
		}
		return false
	}
	return true
}

func safeArchivePath(name string) bool {
	if name == "" || len(name) > 4096 || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return false
	}
	if name == "." || strings.HasPrefix(name, "../") {
		return false
	}
	for _, character := range name {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
