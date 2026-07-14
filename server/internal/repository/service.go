// Copyright 2026 gitsss Authors. All rights reserved.

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidPath    = errors.New("invalid repository path")
	ErrInvalidQuery   = errors.New("invalid search query")
	ErrInvalidRef     = errors.New("invalid repository ref")
	ErrNotFound       = errors.New("repository not found")
	ErrResultTooLarge = errors.New("repository result exceeds the configured limit")
)

const (
	defaultPreviewLimit = 1 << 20
	gitOutputLimit      = 16 << 20
	searchOutputLimit   = 2 << 20
	maxSearchLimit      = 100
)

type Repository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	Path          string `json:"-"`
}

type Entry struct {
	Path        string `json:"path"`
	ObjectID    string `json:"object_id"`
	Mode        string `json:"mode"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	BinaryAsset bool   `json:"binary_asset"`
}

type Tree struct {
	Repository Repository `json:"repository"`
	Ref        string     `json:"ref"`
	Commit     string     `json:"commit"`
	Entries    []Entry    `json:"entries"`
}

type File struct {
	Path             string `json:"path"`
	ObjectID         string `json:"object_id"`
	Size             int64  `json:"size"`
	Binary           bool   `json:"binary"`
	BinaryAsset      bool   `json:"binary_asset"`
	PreviewAvailable bool   `json:"preview_available"`
	Content          string `json:"content,omitempty"`
}

type Match struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type Service struct {
	repositories map[string]Repository
	previewLimit int64
	timeout      time.Duration
}

func NewService(repositories []Repository) (*Service, error) {
	service := &Service{
		repositories: make(map[string]Repository, len(repositories)),
		previewLimit: defaultPreviewLimit,
		timeout:      5 * time.Second,
	}
	for _, repository := range repositories {
		if strings.TrimSpace(repository.ID) == "" || strings.TrimSpace(repository.Name) == "" || strings.TrimSpace(repository.Path) == "" {
			return nil, fmt.Errorf("%w: repository id, name and path are required", ErrInvalidPath)
		}
		absolutePath, err := filepath.Abs(repository.Path)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPath, err)
		}
		repository.Path = absolutePath
		if repository.DefaultBranch == "" {
			repository.DefaultBranch = "HEAD"
		}
		ctx, cancel := context.WithTimeout(context.Background(), service.timeout)
		root, rootErr := runGit(ctx, repository.Path, 4096, "rev-parse", "--show-toplevel")
		cancel()
		if rootErr == nil {
			repository.Path = strings.TrimSpace(string(root))
		}
		ctx, cancel = context.WithTimeout(context.Background(), service.timeout)
		_, err = runGit(ctx, repository.Path, 512, "rev-parse", "--git-dir")
		cancel()
		if err != nil {
			return nil, fmt.Errorf("%w: %s is not a Git repository", ErrInvalidPath, repository.Path)
		}
		service.repositories[repository.ID] = repository
	}
	return service, nil
}

func (s *Service) Repositories() []Repository {
	repositories := make([]Repository, 0, len(s.repositories))
	for _, repository := range s.repositories {
		repositories = append(repositories, repository)
	}
	sort.Slice(repositories, func(i, j int) bool { return repositories[i].Name < repositories[j].Name })
	return repositories
}

func (s *Service) Tree(ctx context.Context, repositoryID, ref string) (Tree, error) {
	repository, err := s.repository(repositoryID)
	if err != nil {
		return Tree{}, err
	}
	commit, err := s.resolveCommit(ctx, repository, ref)
	if err != nil {
		return Tree{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	output, err := runGit(ctx, repository.Path, gitOutputLimit, "ls-tree", "-r", "-z", "--long", commit)
	if err != nil {
		return Tree{}, err
	}

	records := bytes.Split(output, []byte{0})
	entries := make([]Entry, 0, len(records)-1)
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		entry, parseErr := parseTreeEntry(record)
		if parseErr != nil {
			return Tree{}, parseErr
		}
		entries = append(entries, entry)
	}
	return Tree{Repository: repository, Ref: normalizedRef(ref), Commit: commit, Entries: entries}, nil
}

func (s *Service) File(ctx context.Context, repositoryID, ref, filePath string) (File, error) {
	repository, err := s.repository(repositoryID)
	if err != nil {
		return File{}, err
	}
	if err := validateFilePath(filePath); err != nil {
		return File{}, err
	}
	commit, err := s.resolveCommit(ctx, repository, ref)
	if err != nil {
		return File{}, err
	}
	objectSpec := commit + ":" + filePath

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	objectIDBytes, err := runGit(ctx, repository.Path, 256, "rev-parse", "--verify", "--end-of-options", objectSpec)
	if err != nil {
		return File{}, ErrNotFound
	}
	objectID := strings.TrimSpace(string(objectIDBytes))
	sizeBytes, err := runGit(ctx, repository.Path, 256, "cat-file", "-s", objectID)
	if err != nil {
		return File{}, ErrNotFound
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeBytes)), 10, 64)
	if err != nil {
		return File{}, fmt.Errorf("parse Git object size: %w", err)
	}

	file := File{Path: filePath, ObjectID: objectID, Size: size, BinaryAsset: isBinaryAsset(filePath)}
	if size > s.previewLimit {
		file.Binary = file.BinaryAsset
		return file, nil
	}
	content, err := runGit(ctx, repository.Path, s.previewLimit+1, "cat-file", "blob", objectID)
	if err != nil {
		return File{}, err
	}
	file.Binary = bytes.IndexByte(content, 0) >= 0
	file.PreviewAvailable = !file.Binary
	if file.PreviewAvailable {
		file.Content = string(content)
	}
	return file, nil
}

func (s *Service) Search(ctx context.Context, repositoryID, ref, query string, limit int) ([]Match, error) {
	repository, err := s.repository(repositoryID)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 120 || strings.ContainsRune(query, 0) {
		return nil, ErrInvalidQuery
	}
	if limit < 1 {
		limit = 30
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	commit, err := s.resolveCommit(ctx, repository, ref)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	output, err := runGit(ctx, repository.Path, searchOutputLimit, "grep", "--null", "--line-number", "-I", "--max-count=20", "-e", query, commit, "--")
	if err != nil {
		var commandErr *gitCommandError
		if errors.As(err, &commandErr) && commandErr.ExitCode == 1 {
			return []Match{}, nil
		}
		return nil, err
	}

	matches := make([]Match, 0, limit)
	for _, record := range bytes.Split(output, []byte{'\n'}) {
		fields := bytes.SplitN(record, []byte{0}, 3)
		if len(fields) != 3 {
			continue
		}
		line, parseErr := strconv.Atoi(string(fields[1]))
		if parseErr != nil {
			continue
		}
		resultPath := strings.TrimPrefix(string(fields[0]), commit+":")
		matches = append(matches, Match{Path: resultPath, Line: line, Content: string(fields[2])})
		if len(matches) == limit {
			break
		}
	}
	return matches, nil
}

func (s *Service) repository(id string) (Repository, error) {
	repository, ok := s.repositories[id]
	if !ok {
		return Repository{}, ErrNotFound
	}
	return repository, nil
}

func (s *Service) resolveCommit(ctx context.Context, repository Repository, ref string) (string, error) {
	ref = normalizedRef(ref)
	if len(ref) > 255 || strings.ContainsRune(ref, 0) {
		return "", ErrInvalidRef
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	output, err := runGit(ctx, repository.Path, 256, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", ErrInvalidRef
	}
	commit := strings.TrimSpace(string(output))
	if len(commit) != 40 && len(commit) != 64 {
		return "", ErrInvalidRef
	}
	return commit, nil
}

func normalizedRef(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return "HEAD"
	}
	return strings.TrimSpace(ref)
}

func validateFilePath(filePath string) error {
	if filePath == "" || strings.ContainsRune(filePath, 0) || strings.Contains(filePath, "\\") || path.IsAbs(filePath) || path.Clean(filePath) != filePath || filePath == "." || filePath == ".." || strings.HasPrefix(filePath, "../") {
		return ErrInvalidPath
	}
	return nil
}

func parseTreeEntry(record []byte) (Entry, error) {
	metadataAndPath := bytes.SplitN(record, []byte{'\t'}, 2)
	if len(metadataAndPath) != 2 {
		return Entry{}, errors.New("invalid git ls-tree record")
	}
	metadata := strings.Fields(string(metadataAndPath[0]))
	if len(metadata) != 4 {
		return Entry{}, errors.New("invalid git ls-tree metadata")
	}
	size := int64(0)
	if metadata[3] != "-" {
		parsedSize, err := strconv.ParseInt(metadata[3], 10, 64)
		if err != nil {
			return Entry{}, fmt.Errorf("parse git tree size: %w", err)
		}
		size = parsedSize
	}
	entryPath := string(metadataAndPath[1])
	kind := "file"
	if metadata[1] == "commit" {
		kind = "submodule"
	}
	return Entry{Path: entryPath, ObjectID: metadata[2], Mode: metadata[0], Kind: kind, Size: size, BinaryAsset: isBinaryAsset(entryPath)}, nil
}

func isBinaryAsset(filePath string) bool {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".psd", ".psb", ".fbx", ".blend", ".wav", ".mp3", ".ogg", ".flac", ".mp4", ".mov", ".mkv", ".webm":
		return true
	default:
		return false
	}
}

type gitCommandError struct {
	ExitCode int
	Message  string
}

func (e *gitCommandError) Error() string {
	return e.Message
}

func runGit(ctx context.Context, repositoryPath string, outputLimit int64, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repositoryPath}, args...)...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	output, readErr := io.ReadAll(io.LimitReader(stdout, outputLimit+1))
	if int64(len(output)) > outputLimit {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, ErrResultTooLarge
	}
	waitErr := command.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return nil, &gitCommandError{ExitCode: exitCode, Message: message}
	}
	return output, nil
}
