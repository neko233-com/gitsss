// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrConfirmation = errors.New("backup restore confirmation does not match")
	ErrTargetExists = errors.New("backup restore target already exists")
)

type RestoreResult struct {
	AssessmentID string    `json:"assessment_id"`
	Target       string    `json:"target"`
	ContentPath  string    `json:"content_path"`
	Files        int       `json:"files"`
	Bytes        int64     `json:"bytes"`
	RestoredAt   time.Time `json:"restored_at"`
}

func RestoreIsolated(inputPath, passphrase, target, confirmation string) (RestoreResult, error) {
	return RestoreIsolatedExact(inputPath, passphrase, target, confirmation, "")
}

func RestoreIsolatedExact(inputPath, passphrase, target, confirmation, expectedContainerSHA256 string) (RestoreResult, error) {
	verification, err := Verify(inputPath, passphrase)
	if err != nil {
		return RestoreResult{}, err
	}
	if expectedContainerSHA256 != "" && verification.ContainerSHA256 != expectedContainerSHA256 {
		return RestoreResult{}, ErrIntegrity
	}
	if confirmation == "" || confirmation != verification.Manifest.Source.AssessmentID {
		return RestoreResult{}, ErrConfirmation
	}
	if strings.TrimSpace(target) == "" {
		return RestoreResult{}, fmt.Errorf("%w: restore target is required", ErrInvalidBackup)
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return RestoreResult{}, err
	}
	if _, err := os.Lstat(absolute); err == nil {
		return RestoreResult{}, ErrTargetExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return RestoreResult{}, err
	}
	plan, err := BuildImportPlan(verification, absolute, time.Now())
	if err != nil {
		return RestoreResult{}, err
	}
	if plan.Summary.Blocked != 0 || plan.Summary.Conflicts != 0 || plan.Summary.Identical != 0 || plan.Summary.New != len(plan.Items) {
		return RestoreResult{}, fmt.Errorf("%w: target is not a clean isolated destination", ErrInvalidBackup)
	}
	parent := filepath.Dir(absolute)
	parentInfo, err := os.Stat(parent)
	if err != nil || !parentInfo.IsDir() {
		return RestoreResult{}, fmt.Errorf("%w: restore parent does not exist", ErrInvalidBackup)
	}
	temporary, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".restore-*")
	if err != nil {
		return RestoreResult{}, err
	}
	completed := false
	defer func() {
		if !completed {
			removeRestoreTemporary(parent, temporary)
		}
	}()
	if err := os.Chmod(temporary, 0o700); err != nil {
		return RestoreResult{}, err
	}
	secondVerification, err := processContainer(inputPath, passphrase, func(entry Entry, content io.Reader) error {
		return restoreEntry(temporary, entry, content)
	})
	if err != nil {
		return RestoreResult{}, err
	}
	if secondVerification.Manifest.Source.AssessmentID != confirmation {
		return RestoreResult{}, ErrConfirmation
	}
	if secondVerification.ContainerSHA256 != verification.ContainerSHA256 || (expectedContainerSHA256 != "" && secondVerification.ContainerSHA256 != expectedContainerSHA256) {
		return RestoreResult{}, ErrIntegrity
	}
	if err := restoreBareRepositoryDirectories(temporary, secondVerification.Manifest); err != nil {
		return RestoreResult{}, err
	}
	if reason := inspectImportAncestors(absolute); reason != "" {
		return RestoreResult{}, fmt.Errorf("%w: %s", ErrInvalidBackup, reason)
	}
	if err := os.Mkdir(absolute, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return RestoreResult{}, ErrTargetExists
		}
		return RestoreResult{}, err
	}
	contentPath := filepath.Join(absolute, "content")
	if err := os.Rename(temporary, contentPath); err != nil {
		os.Remove(absolute)
		return RestoreResult{}, err
	}
	completed = true
	return RestoreResult{
		AssessmentID: confirmation, Target: absolute, ContentPath: contentPath,
		Files: secondVerification.Files, Bytes: secondVerification.Bytes, RestoredAt: time.Now().UTC(),
	}, nil
}

func restoreBareRepositoryDirectories(root string, manifest Manifest) error {
	repositories, _ := repositoryRoots(manifest)
	for _, repository := range repositories {
		repositoryRoot := filepath.Join(root, filepath.FromSlash(repository))
		relative, err := filepath.Rel(root, repositoryRoot)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: repository path escapes target", ErrInvalidBackup)
		}
		for _, directory := range []string{"objects/info", "objects/pack", "refs/heads", "refs/tags"} {
			if err := os.MkdirAll(filepath.Join(repositoryRoot, filepath.FromSlash(directory)), 0o700); err != nil {
				return err
			}
		}
	}
	return nil
}

func restoreEntry(root string, entry Entry, content io.Reader) error {
	destination := filepath.Join(root, filepath.FromSlash(entry.Path))
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: restore path escapes target", ErrInvalidBackup)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, content)
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if written != entry.Size {
		return ErrIntegrity
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	modified := time.Unix(entry.Modified, 0)
	if err := os.Chtimes(destination, modified, modified); err != nil {
		return err
	}
	return nil
}

func removeRestoreTemporary(parent, temporary string) {
	if filepath.Dir(temporary) != parent || !strings.Contains(filepath.Base(temporary), ".restore-") {
		return
	}
	os.RemoveAll(temporary)
}
