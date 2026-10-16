// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package repolayout records the on-disk contract for Git repository data.
// Upgrades may add files under the extensions directory. They must not move or
// rewrite bare repositories, Git objects, refs, or LFS object bytes.
package repolayout

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/neko233-com/gitsss/modules/json"
)

const (
	// Version is the layout contract implemented by this binary.
	Version = 1
	// ContractGitObjectsImmutable is the only accepted contract name.
	ContractGitObjectsImmutable = "git-objects-immutable"

	// RoleRepositories is the frozen bare-repository tree.
	RoleRepositories = "git-repositories"
	// RoleLFS is the frozen Git LFS object store.
	RoleLFS = "git-lfs"
	// RoleDatabase is the frozen application database location.
	RoleDatabase = "database"

	sidecarDir    = "gitsss"
	manifestName  = "layout.json"
	extensionsDir = "extensions"
)

// ErrFutureLayout means a newer gitsss already recorded a layout this binary
// does not understand. Callers must stop without changing Git or LFS data.
var ErrFutureLayout = errors.New("git layout is newer than this gitsss version")

// Manifest is the sidecar stored beside repository data.
type Manifest struct {
	LayoutVersion int               `json:"layout_version"`
	Contract      string            `json:"contract"`
	Frozen        []string          `json:"frozen"`
	ExtensionsDir string            `json:"extensions_dir"`
	Reserved      map[string]string `json:"reserved,omitempty"`
}

// RequiredFrozen returns the role names every layout v1 file must contain.
func RequiredFrozen() []string {
	return []string{RoleRepositories, RoleLFS, RoleDatabase}
}

// ManifestPath returns the sidecar path under the application data root.
func ManifestPath(dataRoot string) string {
	return filepath.Join(dataRoot, sidecarDir, manifestName)
}

// ExtensionsPath returns the directory future upgrades may populate.
func ExtensionsPath(dataRoot string) string {
	return filepath.Join(dataRoot, sidecarDir, extensionsDir)
}

// Read loads the sidecar without creating or changing it.
// A missing frozen role or a newer layout version is an error.
func Read(dataRoot string) (Manifest, error) {
	body, err := os.ReadFile(ManifestPath(dataRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{}, errors.New("git layout sidecar is missing; start gitsss or run doctor --fix to record it without moving repositories")
		}
		return Manifest{}, err
	}
	_, manifest, err := readManifest(body)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.LayoutVersion > Version {
		return Manifest{}, fmt.Errorf("%w: found %d, this binary understands %d", ErrFutureLayout, manifest.LayoutVersion, Version)
	}
	if manifest.Contract != "" && manifest.Contract != ContractGitObjectsImmutable {
		return Manifest{}, fmt.Errorf("git layout contract %q is not supported", manifest.Contract)
	}
	for _, role := range RequiredFrozen() {
		if !contains(manifest.Frozen, role) {
			return Manifest{}, fmt.Errorf("git layout is missing frozen role %s", role)
		}
	}
	return manifest, nil
}

// Ensure creates or updates the sidecar. It refuses a newer layout and does
// not create, move, or open repository or LFS trees.
func Ensure(dataRoot, repoRoot, lfsRoot string) (Manifest, error) {
	ext := ExtensionsPath(dataRoot)
	if err := rejectOverlap(ext, repoRoot, lfsRoot); err != nil {
		return Manifest{}, err
	}
	path := ManifestPath(dataRoot)
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Manifest{}, err
	}
	if errors.Is(err, os.ErrNotExist) {
		manifest := newManifest()
		if err := writeManifest(path, nil, manifest); err != nil {
			return Manifest{}, err
		}
		if err := os.MkdirAll(ext, 0o755); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}
	raw, manifest, err := readManifest(body)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.LayoutVersion > Version {
		return Manifest{}, fmt.Errorf("%w: found %d, this binary understands %d", ErrFutureLayout, manifest.LayoutVersion, Version)
	}
	if manifest.Contract != "" && manifest.Contract != ContractGitObjectsImmutable {
		return Manifest{}, fmt.Errorf("git layout contract %q is not supported", manifest.Contract)
	}
	updated := mergeManifest(manifest)
	if !sameManifest(manifest, updated) {
		if err := writeManifest(path, raw, updated); err != nil {
			return Manifest{}, err
		}
	}
	if err := os.MkdirAll(ext, 0o755); err != nil {
		return Manifest{}, err
	}
	return updated, nil
}

// ReserveExtension records an additive feature directory. The id is a single
// path segment. The returned directory is always inside the extensions tree.
func ReserveExtension(dataRoot, repoRoot, lfsRoot, id string) (string, error) {
	if err := validateExtensionID(id); err != nil {
		return "", err
	}
	if _, err := Ensure(dataRoot, repoRoot, lfsRoot); err != nil {
		return "", err
	}
	dir := filepath.Join(ExtensionsPath(dataRoot), id)
	if err := rejectOverlap(dir, repoRoot, lfsRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	marker := filepath.Join(dir, "extension.json")
	if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
		payload, err := json.Marshal(map[string]string{"id": id, "layout_version": strconv.Itoa(Version)})
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(marker, append(payload, '\n'), 0o644); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if err := rememberExtension(dataRoot, id); err != nil {
		return "", err
	}
	return dir, nil
}

func newManifest() Manifest {
	return Manifest{
		LayoutVersion: Version,
		Contract:      ContractGitObjectsImmutable,
		Frozen:        RequiredFrozen(),
		ExtensionsDir: filepath.ToSlash(filepath.Join(sidecarDir, extensionsDir)),
		Reserved:      map[string]string{},
	}
}

func mergeManifest(current Manifest) Manifest {
	next := newManifest()
	seen := map[string]struct{}{}
	for _, role := range append(append([]string{}, next.Frozen...), current.Frozen...) {
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		if !contains(next.Frozen, role) {
			next.Frozen = append(next.Frozen, role)
		}
	}
	if current.Reserved != nil {
		next.Reserved = make(map[string]string, len(current.Reserved))
		maps.Copy(next.Reserved, current.Reserved)
	}
	return next
}

func sameManifest(left, right Manifest) bool {
	if left.LayoutVersion != right.LayoutVersion || left.Contract != right.Contract || left.ExtensionsDir != right.ExtensionsDir {
		return false
	}
	if len(left.Frozen) != len(right.Frozen) {
		return false
	}
	for i := range left.Frozen {
		if left.Frozen[i] != right.Frozen[i] {
			return false
		}
	}
	if len(left.Reserved) != len(right.Reserved) {
		return false
	}
	for key, value := range left.Reserved {
		if right.Reserved[key] != value {
			return false
		}
	}
	return true
}

func readManifest(body []byte) (map[string]any, Manifest, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, Manifest{}, err
	}
	return raw, manifest, nil
}

func writeManifest(path string, raw map[string]any, manifest Manifest) error {
	if raw == nil {
		raw = map[string]any{}
	}
	raw["layout_version"] = manifest.LayoutVersion
	raw["contract"] = manifest.Contract
	raw["frozen"] = manifest.Frozen
	raw["extensions_dir"] = manifest.ExtensionsDir
	if len(manifest.Reserved) == 0 {
		delete(raw, "reserved")
	} else {
		raw["reserved"] = manifest.Reserved
	}
	body, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "layout-*.json")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	remove := true
	defer func() {
		_ = temp.Close()
		if remove {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(body); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempName, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	remove = false
	return nil
}

func rememberExtension(dataRoot, id string) error {
	path := ManifestPath(dataRoot)
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw, manifest, err := readManifest(body)
	if err != nil {
		return err
	}
	if manifest.LayoutVersion > Version {
		return fmt.Errorf("%w: found %d, this binary understands %d", ErrFutureLayout, manifest.LayoutVersion, Version)
	}
	manifest = mergeManifest(manifest)
	if manifest.Reserved == nil {
		manifest.Reserved = map[string]string{}
	}
	manifest.Reserved[id] = filepath.ToSlash(filepath.Join(sidecarDir, extensionsDir, id))
	return writeManifest(path, raw, manifest)
}

func validateExtensionID(id string) error {
	if id == "" || len(id) > 64 {
		return fmt.Errorf("invalid layout extension id %q", id)
	}
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' && i > 0:
		default:
			return fmt.Errorf("invalid layout extension id %q", id)
		}
	}
	return nil
}

func rejectOverlap(ext, repoRoot, lfsRoot string) error {
	for _, root := range []string{repoRoot, lfsRoot} {
		if overlaps(ext, root) {
			return fmt.Errorf("git layout extensions overlap frozen data at %s", root)
		}
	}
	return nil
}

func overlaps(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if left == right {
		return true
	}
	if inside(left, right) || inside(right, left) {
		return true
	}
	return false
}

func inside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}

// hashTree returns a stable digest of regular files under root.
// Missing roots hash as empty. Tests use it to prove upgrades do not rewrite Git data.
func hashTree(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing", nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", root)
	}
	digest := ""
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest += filepath.ToSlash(rel) + "\x00" + string(body) + "\x00"
		return nil
	})
	return digest, err
}
