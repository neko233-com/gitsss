// Copyright 2026 gitsss Authors. All rights reserved.

package launcher

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	Format       = "gitsss-launcher-v1"
	maxStateSize = 64 << 10
)

var (
	ErrInvalidState = errors.New("invalid launcher state")
	stateName       = regexp.MustCompile(`^active-([0-9]{20})\.json$`)
)

type State struct {
	Format           string   `json:"format"`
	Generation       uint64   `json:"generation"`
	Version          string   `json:"version"`
	Commit           string   `json:"commit"`
	Platform         string   `json:"platform"`
	Architecture     string   `json:"architecture"`
	Directory        string   `json:"directory"`
	Executable       string   `json:"executable"`
	WorkingDirectory string   `json:"working_directory"`
	Arguments        []string `json:"arguments"`
}

func Canonical(state State) ([]byte, error) {
	state.Arguments = append([]string(nil), state.Arguments...)
	if err := Validate(state); err != nil {
		return nil, err
	}
	content, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func Validate(state State) error {
	if state.Format != Format || state.Generation == 0 || !safeValue(state.Version) || !validCommit(state.Commit) || !validTarget(state.Platform, state.Architecture) {
		return fmt.Errorf("%w: header is invalid", ErrInvalidState)
	}
	if !cleanTargetAbsolute(state.Platform, state.Directory) || !cleanTargetAbsolute(state.Platform, state.WorkingDirectory) {
		return fmt.Errorf("%w: directories must be clean absolute paths", ErrInvalidState)
	}
	expectedExecutable := "gitsss-agent"
	if state.Platform == "windows" {
		expectedExecutable += ".exe"
	}
	if state.Executable != expectedExecutable || filepath.Base(state.Executable) != state.Executable {
		return fmt.Errorf("%w: executable must be %s", ErrInvalidState, expectedExecutable)
	}
	if len(state.Arguments) == 0 || len(state.Arguments) > 128 {
		return fmt.Errorf("%w: arguments are required", ErrInvalidState)
	}
	for _, argument := range state.Arguments {
		if argument == "" || len(argument) > 4096 || strings.ContainsAny(argument, "\x00\r\n") {
			return fmt.Errorf("%w: argument is invalid", ErrInvalidState)
		}
	}
	return nil
}

func ExecutablePath(state State) (string, error) {
	if err := Validate(state); err != nil {
		return "", err
	}
	if state.Platform != runtime.GOOS || state.Architecture != runtime.GOARCH {
		return "", fmt.Errorf("%w: state target %s/%s does not match host %s/%s", ErrInvalidState, state.Platform, state.Architecture, runtime.GOOS, runtime.GOARCH)
	}
	for _, directory := range []string{state.Directory, state.WorkingDirectory} {
		info, err := os.Lstat(directory)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("%w: runtime directory is not a regular directory", ErrInvalidState)
		}
	}
	filePath := filepath.Join(state.Directory, state.Executable)
	info, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: executable is not a regular file", ErrInvalidState)
	}
	return filePath, nil
}

func LoadLatest(directory string) (State, string, error) {
	if !cleanAbsolute(directory) {
		return State{}, "", fmt.Errorf("%w: state directory must be a clean absolute path", ErrInvalidState)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return State{}, "", err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && stateName.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return State{}, "", fmt.Errorf("%w: no active state", ErrInvalidState)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	filePath := filepath.Join(directory, names[0])
	content, err := readStateFile(filePath)
	if err != nil {
		return State{}, "", err
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, "", fmt.Errorf("%w: %v", ErrInvalidState, err)
	}
	canonical, err := Canonical(state)
	if err != nil {
		return State{}, "", err
	}
	if !bytes.Equal(content, canonical) || StateFilename(state.Generation) != names[0] {
		return State{}, "", fmt.Errorf("%w: state is not canonical or generation mismatches", ErrInvalidState)
	}
	return state, filePath, nil
}

func Publish(directory string, state State) (string, error) {
	if !cleanAbsolute(directory) {
		return "", fmt.Errorf("%w: state directory must be a clean absolute path", ErrInvalidState)
	}
	content, err := Canonical(state)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: state directory does not exist", ErrInvalidState)
	}
	filePath := filepath.Join(directory, StateFilename(state.Generation))
	if err := atomicStateFile(filePath, content); err != nil {
		return "", err
	}
	return filePath, nil
}

func StateFilename(generation uint64) string {
	return fmt.Sprintf("active-%020d.json", generation)
}

func readStateFile(filePath string) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxStateSize {
		return nil, fmt.Errorf("%w: state must be a regular file", ErrInvalidState)
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
		return nil, fmt.Errorf("%w: state changed while opening", ErrInvalidState)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxStateSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxStateSize {
		return nil, fmt.Errorf("%w: state is too large", ErrInvalidState)
	}
	return content, nil
}

func atomicStateFile(filePath string, content []byte) error {
	directory := filepath.Dir(filePath)
	temporary, err := os.CreateTemp(directory, ".gitsss-launcher-partial-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	linked := false
	completed := false
	defer func() {
		if !completed {
			temporary.Close()
			_ = os.Remove(temporaryPath)
			if linked {
				_ = os.Remove(filePath)
			}
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, filePath); err != nil {
		return err
	}
	linked = true
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		directoryHandle, err := os.Open(directory)
		if err != nil {
			return err
		}
		if err := directoryHandle.Sync(); err != nil {
			directoryHandle.Close()
			return err
		}
		if err := directoryHandle.Close(); err != nil {
			return err
		}
	}
	completed = true
	return nil
}

func cleanAbsolute(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func cleanTargetAbsolute(platform, value string) bool {
	if value == "" || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	if platform != "windows" {
		return strings.HasPrefix(value, "/") && path.Clean(value) == value
	}
	normalized := strings.ReplaceAll(value, `\`, "/")
	if len(normalized) < 3 || normalized[1] != ':' || normalized[2] != '/' || !((normalized[0] >= 'A' && normalized[0] <= 'Z') || (normalized[0] >= 'a' && normalized[0] <= 'z')) {
		return false
	}
	return path.Clean(normalized) == normalized
}

func safeValue(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '.' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validCommit(value string) bool {
	if len(value) != 40 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validTarget(platform, architecture string) bool {
	return (platform == "windows" || platform == "darwin" || platform == "linux") && (architecture == "amd64" || architecture == "arm64")
}
