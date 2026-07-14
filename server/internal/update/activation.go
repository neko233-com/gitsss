// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/launcher"
)

var ErrActivationFailed = errors.New("update activation failed")

const activationLockName = ".gitsss-update-activation.lock"

type ActivationHost interface {
	Restart(context.Context, launcher.State) error
	Healthy(context.Context, launcher.State) error
}

type ActivationResult struct {
	Version            string `json:"version"`
	Commit             string `json:"commit"`
	PreviousGeneration uint64 `json:"previous_generation"`
	ActiveGeneration   uint64 `json:"active_generation"`
	ActiveStatePath    string `json:"active_state_path"`
	RolledBack         bool   `json:"rolled_back"`
	RollbackGeneration uint64 `json:"rollback_generation,omitempty"`
	RollbackStatePath  string `json:"rollback_state_path,omitempty"`
}

func Activate(ctx context.Context, stageDirectory, stateDirectory string, host ActivationHost) (ActivationResult, error) {
	if host == nil {
		return ActivationResult{}, fmt.Errorf("%w: activation host is required", ErrActivationFailed)
	}
	lockPath, err := acquireActivationLock(stateDirectory)
	if err != nil {
		return ActivationResult{}, err
	}
	result, activationErr := activateLocked(ctx, stageDirectory, stateDirectory, host)
	if releaseErr := os.Remove(lockPath); releaseErr != nil {
		return result, errors.Join(activationErr, fmt.Errorf("%w: release activation lock: %v", ErrActivationFailed, releaseErr))
	}
	return result, activationErr
}

func activateLocked(ctx context.Context, stageDirectory, stateDirectory string, host ActivationHost) (ActivationResult, error) {
	previous, _, err := launcher.LoadLatest(stateDirectory)
	if err != nil {
		return ActivationResult{}, err
	}
	if previous.Generation >= math.MaxUint64-1 {
		return ActivationResult{}, fmt.Errorf("%w: launcher generation is exhausted", ErrActivationFailed)
	}
	if _, err := launcher.ExecutablePath(previous); err != nil {
		return ActivationResult{}, fmt.Errorf("%w: previous version is not runnable: %v", ErrActivationFailed, err)
	}
	staged, err := LoadStage(stageDirectory)
	if err != nil {
		return ActivationResult{}, err
	}
	if staged.Artifact.OS != previous.Platform || staged.Artifact.Arch != previous.Architecture || staged.Artifact.OS != runtime.GOOS || staged.Artifact.Arch != runtime.GOARCH {
		return ActivationResult{}, fmt.Errorf("%w: staged target does not match active host", ErrActivationFailed)
	}
	active := previous
	active.Generation++
	active.Version = staged.Version
	active.Commit = staged.Commit
	active.Directory = staged.Target
	activeStatePath, err := launcher.Publish(stateDirectory, active)
	if err != nil {
		return ActivationResult{}, fmt.Errorf("%w: publish active state: %v", ErrActivationFailed, err)
	}
	result := ActivationResult{
		Version: staged.Version, Commit: staged.Commit, PreviousGeneration: previous.Generation,
		ActiveGeneration: active.Generation, ActiveStatePath: activeStatePath,
	}
	if _, validationErr := launcher.ExecutablePath(active); validationErr != nil {
		err = validationErr
	} else {
		err = host.Restart(ctx, active)
		if err == nil {
			err = host.Healthy(ctx, active)
		}
	}
	if err == nil {
		return result, nil
	}
	activationErr := fmt.Errorf("activate generation %d: %w", active.Generation, err)
	rollback := previous
	rollback.Generation = active.Generation + 1
	rollbackStatePath, rollbackPublishErr := launcher.Publish(stateDirectory, rollback)
	result.RollbackGeneration = rollback.Generation
	result.RollbackStatePath = rollbackStatePath
	if rollbackPublishErr != nil {
		return result, errors.Join(ErrActivationFailed, activationErr, fmt.Errorf("publish rollback state: %w", rollbackPublishErr))
	}
	result.RolledBack = true
	rollbackRestartErr := host.Restart(ctx, rollback)
	var rollbackHealthErr error
	if rollbackRestartErr == nil {
		rollbackHealthErr = host.Healthy(ctx, rollback)
	}
	if rollbackRestartErr != nil || rollbackHealthErr != nil {
		return result, errors.Join(ErrActivationFailed, activationErr, wrapActivationError("restart rollback", rollbackRestartErr), wrapActivationError("verify rollback", rollbackHealthErr))
	}
	return result, errors.Join(ErrActivationFailed, activationErr)
}

func acquireActivationLock(stateDirectory string) (string, error) {
	if !cleanActivationPath(stateDirectory) {
		return "", fmt.Errorf("%w: state directory must be a clean absolute path", ErrActivationFailed)
	}
	info, err := os.Lstat(stateDirectory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%w: state directory is not a regular directory", ErrActivationFailed)
	}
	lockPath := filepath.Join(stateDirectory, activationLockName)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("%w: another activation is running or its lock requires recovery: %v", ErrActivationFailed, err)
	}
	content := fmt.Sprintf("pid=%d started_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(lockPath)
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(lockPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(lockPath)
		return "", err
	}
	return lockPath, nil
}

func LoadStage(stageDirectory string) (StageResult, error) {
	if !cleanActivationPath(stageDirectory) {
		return StageResult{}, fmt.Errorf("%w: staging directory must be a clean absolute path", ErrActivationFailed)
	}
	info, err := os.Lstat(stageDirectory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return StageResult{}, fmt.Errorf("%w: staging directory is not a regular directory", ErrActivationFailed)
	}
	content, err := readActivationFile(filepath.Join(stageDirectory, stageReadyName), 64<<10)
	if err != nil {
		return StageResult{}, err
	}
	var result StageResult
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return StageResult{}, fmt.Errorf("%w: invalid stage metadata: %v", ErrActivationFailed, err)
	}
	canonical, err := json.Marshal(result)
	if err != nil {
		return StageResult{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(content, canonical) || result.Target != stageDirectory || result.Files <= 0 || result.Files > maxStageEntries || result.Bytes < 0 || result.Bytes > maxStageBytes || !validActivationKeyID(result.KeyID) {
		return StageResult{}, fmt.Errorf("%w: stage metadata is not canonical or bounded", ErrActivationFailed)
	}
	stagedAt, err := time.Parse(time.RFC3339, result.StagedAt)
	if err != nil || stagedAt.Location() != time.UTC || stagedAt.Format(time.RFC3339) != result.StagedAt {
		return StageResult{}, fmt.Errorf("%w: staged_at must be canonical UTC RFC3339", ErrActivationFailed)
	}
	if _, err := Canonical(Manifest{
		Format: Format, Version: result.Version, Commit: result.Commit,
		PublishedAt: result.PublishedAt, Artifacts: []Artifact{result.Artifact},
	}); err != nil {
		return StageResult{}, fmt.Errorf("%w: %v", ErrActivationFailed, err)
	}
	if err := validateStageBuildInfo(stageDirectory, result); err != nil {
		return StageResult{}, err
	}
	return result, nil
}

func validateStageBuildInfo(stageDirectory string, staged StageResult) error {
	content, err := readActivationFile(filepath.Join(stageDirectory, "BUILDINFO.json"), 16<<10)
	if err != nil {
		return err
	}
	var build struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		GOOS    string `json:"goos"`
		GOARCH  string `json:"goarch"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&build); err != nil {
		return fmt.Errorf("%w: invalid BUILDINFO.json: %v", ErrActivationFailed, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: BUILDINFO.json has trailing data", ErrActivationFailed)
	}
	if build.Version != staged.Version || build.Commit != staged.Commit || build.GOOS != staged.Artifact.OS || build.GOARCH != staged.Artifact.Arch {
		return fmt.Errorf("%w: BUILDINFO.json differs from signed stage metadata", ErrActivationFailed)
	}
	return nil
}

func readActivationFile(filePath string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximum {
		return nil, fmt.Errorf("%w: activation metadata must be a bounded regular file", ErrActivationFailed)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	currentInfo, err := os.Lstat(filePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		return nil, fmt.Errorf("%w: activation metadata changed while opening", ErrActivationFailed)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(content)) > maximum {
		return nil, fmt.Errorf("%w: activation metadata is unreadable or too large", ErrActivationFailed)
	}
	return content, nil
}

func validActivationKeyID(value string) bool {
	if len(value) != 16 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cleanActivationPath(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func wrapActivationError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
