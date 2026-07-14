// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

const cutoverStateVersion = 1

var (
	ErrCutoverConflict = errors.New("cutover state conflicts with the requested operation")
	ErrActiveChanged   = errors.New("active data changed after cutover")
)

type CutoverOptions struct {
	AssessmentID    string
	ContainerSHA256 string
	RestoredTarget  string
	ActivePath      string
	Confirmation    string
	Now             func() time.Time
}

type CutoverResult struct {
	ID              string    `json:"id"`
	AssessmentID    string    `json:"assessment_id"`
	ActivePath      string    `json:"active_path"`
	RollbackPath    string    `json:"rollback_path,omitempty"`
	StatePath       string    `json:"state_path"`
	PreviousActive  bool      `json:"previous_active"`
	ActivatedSHA256 string    `json:"activated_sha256"`
	CutoverAt       time.Time `json:"cutover_at"`
}

type RollbackResult struct {
	CutoverID        string    `json:"cutover_id"`
	ActivePath       string    `json:"active_path"`
	ArchivedPath     string    `json:"archived_path"`
	RestoredPrevious bool      `json:"restored_previous"`
	RolledBackAt     time.Time `json:"rolled_back_at"`
}

type cutoverState struct {
	Version         int       `json:"version"`
	ID              string    `json:"id"`
	AssessmentID    string    `json:"assessment_id"`
	ContainerSHA256 string    `json:"container_sha256"`
	ActivePath      string    `json:"active_path"`
	RollbackPath    string    `json:"rollback_path,omitempty"`
	PreviousActive  bool      `json:"previous_active"`
	ActivatedSHA256 string    `json:"activated_sha256"`
	Status          string    `json:"status"`
	CutoverAt       time.Time `json:"cutover_at"`
	RolledBackAt    time.Time `json:"rolled_back_at,omitempty"`
	ArchivedPath    string    `json:"archived_path,omitempty"`
}

func CutoverVerified(options CutoverOptions) (CutoverResult, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if strings.TrimSpace(options.AssessmentID) == "" || !validSHA256(options.ContainerSHA256) ||
		options.Confirmation != "CUTOVER "+options.AssessmentID {
		return CutoverResult{}, ErrConfirmation
	}
	restoredTarget, activePath, err := cutoverPaths(options.RestoredTarget, options.ActivePath)
	if err != nil {
		return CutoverResult{}, err
	}
	contentPath := filepath.Join(restoredTarget, "content")
	if err := requirePlainDirectory(contentPath); err != nil {
		return CutoverResult{}, err
	}
	activatedDigest, err := treeDigest(contentPath)
	if err != nil {
		return CutoverResult{}, err
	}
	statePath := activePath + ".gitsss-cutover.json"
	if _, err := os.Lstat(statePath); err == nil {
		return CutoverResult{}, ErrCutoverConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return CutoverResult{}, err
	}
	now := options.Now().UTC()
	id, err := newCutoverID()
	if err != nil {
		return CutoverResult{}, err
	}
	marker := cutoverState{
		Version: cutoverStateVersion, ID: id, AssessmentID: options.AssessmentID,
		ContainerSHA256: options.ContainerSHA256, ActivePath: activePath,
		ActivatedSHA256: activatedDigest, Status: "prepared", CutoverAt: now,
	}
	if info, statErr := os.Lstat(activePath); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return CutoverResult{}, fmt.Errorf("%w: active path is not a plain directory", ErrCutoverConflict)
		}
		marker.PreviousActive = true
		marker.RollbackPath = activePath + ".rollback-" + id
		if _, err := os.Lstat(marker.RollbackPath); !errors.Is(err, os.ErrNotExist) {
			return CutoverResult{}, ErrCutoverConflict
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return CutoverResult{}, statErr
	}
	if err := writeCutoverState(statePath, marker, true); err != nil {
		return CutoverResult{}, err
	}
	if marker.PreviousActive {
		if err := os.Rename(activePath, marker.RollbackPath); err != nil {
			return CutoverResult{}, err
		}
	}
	if err := os.Rename(contentPath, activePath); err != nil {
		if marker.PreviousActive {
			_ = os.Rename(marker.RollbackPath, activePath)
		}
		return CutoverResult{}, err
	}
	marker.Status = "active"
	if err := writeCutoverState(statePath, marker, false); err != nil {
		return CutoverResult{}, fmt.Errorf("cutover activated but state finalization failed: %w", err)
	}
	return CutoverResult{
		ID: id, AssessmentID: marker.AssessmentID, ActivePath: activePath,
		RollbackPath: marker.RollbackPath, StatePath: statePath, PreviousActive: marker.PreviousActive,
		ActivatedSHA256: activatedDigest, CutoverAt: now,
	}, nil
}

func RollbackCutover(activePath, statePath, cutoverID, confirmation string, now func() time.Time) (RollbackResult, error) {
	if now == nil {
		now = time.Now
	}
	absoluteActive, err := filepath.Abs(activePath)
	if err != nil {
		return RollbackResult{}, err
	}
	absoluteState, err := filepath.Abs(statePath)
	if err != nil {
		return RollbackResult{}, err
	}
	marker, err := readCutoverState(absoluteState)
	if err != nil {
		return RollbackResult{}, err
	}
	if (marker.Status != "active" && marker.Status != "prepared") || marker.ID != cutoverID || marker.ActivePath != filepath.Clean(absoluteActive) || confirmation != "ROLLBACK "+marker.ID {
		return RollbackResult{}, ErrConfirmation
	}
	if marker.Status == "prepared" {
		recovered, complete, err := recoverPreparedCutover(absoluteState, marker, now().UTC())
		if err != nil || complete {
			return recovered, err
		}
		marker.Status = "active"
	}
	if err := requirePlainDirectory(marker.ActivePath); err != nil {
		return RollbackResult{}, err
	}
	digest, err := treeDigest(marker.ActivePath)
	if err != nil {
		return RollbackResult{}, err
	}
	if digest != marker.ActivatedSHA256 {
		return RollbackResult{}, ErrActiveChanged
	}
	if marker.PreviousActive {
		if err := requirePlainDirectory(marker.RollbackPath); err != nil {
			return RollbackResult{}, err
		}
	}
	rolledBackAt := now().UTC()
	archivedPath := marker.ActivePath + ".rolled-back-" + marker.ID
	if _, err := os.Lstat(archivedPath); !errors.Is(err, os.ErrNotExist) {
		return RollbackResult{}, ErrCutoverConflict
	}
	if err := os.Rename(marker.ActivePath, archivedPath); err != nil {
		return RollbackResult{}, err
	}
	if marker.PreviousActive {
		if err := os.Rename(marker.RollbackPath, marker.ActivePath); err != nil {
			_ = os.Rename(archivedPath, marker.ActivePath)
			return RollbackResult{}, err
		}
	}
	marker.Status = "rolled_back"
	marker.RolledBackAt = rolledBackAt
	marker.ArchivedPath = archivedPath
	if err := writeCutoverState(absoluteState, marker, false); err != nil {
		return RollbackResult{}, fmt.Errorf("rollback completed but state finalization failed: %w", err)
	}
	return RollbackResult{
		CutoverID: marker.ID, ActivePath: marker.ActivePath, ArchivedPath: archivedPath,
		RestoredPrevious: marker.PreviousActive, RolledBackAt: rolledBackAt,
	}, nil
}

func cutoverPaths(restoredTarget, activePath string) (string, string, error) {
	restored, err := filepath.Abs(restoredTarget)
	if err != nil {
		return "", "", err
	}
	active, err := filepath.Abs(activePath)
	if err != nil {
		return "", "", err
	}
	restored = filepath.Clean(restored)
	active = filepath.Clean(active)
	if restored == active || pathContains(restored, active) || pathContains(active, restored) || filepath.Dir(active) == active {
		return "", "", fmt.Errorf("%w: cutover paths overlap or target a volume root", ErrCutoverConflict)
	}
	if reason := inspectImportAncestors(restored); reason != "" {
		return "", "", fmt.Errorf("%w: restored target %s", ErrCutoverConflict, reason)
	}
	if reason := inspectImportAncestors(filepath.Dir(active)); reason != "" {
		return "", "", fmt.Errorf("%w: active parent %s", ErrCutoverConflict, reason)
	}
	return restored, active, nil
}

func pathContains(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func requirePlainDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: %s is not a plain directory", ErrCutoverConflict, path)
	}
	return nil
}

func treeDigest(root string) (string, error) {
	hasher := sha256.New()
	entries := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		entries++
		if entries > maxManifestEntries {
			return fmt.Errorf("%w: active tree exceeds entry limit", ErrCutoverConflict)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: active tree contains a symbolic link", ErrCutoverConflict)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hasher, filepath.ToSlash(relative)+"\x00")
		if entry.IsDir() {
			_, _ = io.WriteString(hasher, "d\x00")
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%w: active tree contains a special file", ErrCutoverConflict)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		_, _ = io.WriteString(hasher, "\x00")
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeCutoverState(path string, marker cutoverState, exclusive bool) error {
	payload, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if exclusive {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
		temporary = path
	}
	file, err := os.OpenFile(temporary, flags, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(payload); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if exclusive {
			_ = os.Remove(path)
		} else {
			_ = os.Remove(temporary)
		}
		return err
	}
	if !exclusive {
		if err := replaceCutoverFile(temporary, path); err != nil {
			_ = os.Remove(temporary)
			return err
		}
	}
	return state.ProtectFile(path)
}

func recoverPreparedCutover(statePath string, marker cutoverState, recoveredAt time.Time) (RollbackResult, bool, error) {
	activeInfo, activeErr := os.Lstat(marker.ActivePath)
	rollbackInfo, rollbackErr := os.Lstat(marker.RollbackPath)
	activeExists := activeErr == nil && activeInfo.IsDir() && activeInfo.Mode()&os.ModeSymlink == 0
	rollbackExists := marker.PreviousActive && rollbackErr == nil && rollbackInfo.IsDir() && rollbackInfo.Mode()&os.ModeSymlink == 0
	if activeExists {
		digest, err := treeDigest(marker.ActivePath)
		if err != nil {
			return RollbackResult{}, false, err
		}
		if digest == marker.ActivatedSHA256 {
			marker.Status = "active"
			if err := writeCutoverState(statePath, marker, false); err != nil {
				return RollbackResult{}, false, err
			}
			return RollbackResult{}, false, nil
		}
	}
	if marker.PreviousActive && !activeExists && rollbackExists {
		if err := os.Rename(marker.RollbackPath, marker.ActivePath); err != nil {
			return RollbackResult{}, false, err
		}
		activeExists = true
		rollbackExists = false
	}
	if (marker.PreviousActive && activeExists && !rollbackExists) || (!marker.PreviousActive && !activeExists) {
		marker.Status = "rolled_back"
		marker.RolledBackAt = recoveredAt
		if err := writeCutoverState(statePath, marker, false); err != nil {
			return RollbackResult{}, false, err
		}
		return RollbackResult{
			CutoverID: marker.ID, ActivePath: marker.ActivePath,
			RestoredPrevious: marker.PreviousActive, RolledBackAt: recoveredAt,
		}, true, nil
	}
	return RollbackResult{}, false, ErrCutoverConflict
}

func readCutoverState(path string) (cutoverState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return cutoverState{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 32<<10 {
		return cutoverState{}, ErrCutoverConflict
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return cutoverState{}, err
	}
	var marker cutoverState
	if err := json.Unmarshal(payload, &marker); err != nil || marker.Version != cutoverStateVersion || marker.ID == "" || !validSHA256(marker.ActivatedSHA256) {
		return cutoverState{}, ErrCutoverConflict
	}
	return marker, nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newCutoverID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "cutover-" + hex.EncodeToString(value), nil
}
