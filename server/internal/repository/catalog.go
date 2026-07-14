// Copyright 2026 gitsss Authors. All rights reserved.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

const catalogNamespace = "repository-catalog"

type catalogConfiguration struct {
	root  string
	store *state.Store
	audit *audit.Log
}

type catalogRecord struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"source_id,omitempty"`
	Name          string    `json:"name"`
	DefaultBranch string    `json:"default_branch"`
	Path          string    `json:"path"`
	Hosted        bool      `json:"hosted"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateInput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
}

func NewPersistentService(log *audit.Log, store *state.Store, root string, existing []Repository) (*Service, error) {
	if log == nil || store == nil || strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%w: catalog dependencies are required", ErrState)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	absoluteRoot = filepath.Clean(absoluteRoot)
	if err := secureRepositoryRoot(absoluteRoot); err != nil {
		return nil, err
	}
	service, err := NewService(nil)
	if err != nil {
		return nil, err
	}
	service.catalog = &catalogConfiguration{root: absoluteRoot, store: store, audit: log}
	documents, err := store.List(catalogNamespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	for _, document := range documents {
		var record catalogRecord
		if err := json.Unmarshal(document.Payload, &record); err != nil || record.ID != document.ID {
			return nil, fmt.Errorf("%w: corrupt repository %q", ErrState, document.ID)
		}
		repository, err := service.normalizeRepository(record.repository())
		if err != nil {
			return nil, fmt.Errorf("%w: corrupt repository %q: %v", ErrState, document.ID, err)
		}
		service.repositories[repository.ID] = repository
	}
	for _, candidate := range existing {
		repository, err := service.normalizeRepository(candidate)
		if err != nil {
			return nil, err
		}
		if current, exists := service.repositories[repository.ID]; exists {
			if current.Path != repository.Path {
				return nil, fmt.Errorf("%w: repository %q has a different path", ErrState, repository.ID)
			}
			continue
		}
		if repository.CreatedAt.IsZero() {
			repository.CreatedAt = time.Now().UTC()
		}
		if err := service.persist(repository); err != nil {
			return nil, err
		}
		service.repositories[repository.ID] = repository
	}
	return service, nil
}

func (s *Service) Create(actor, requestID string, input CreateInput, at time.Time) (Repository, error) {
	if s.catalog == nil || strings.TrimSpace(actor) == "" || strings.TrimSpace(requestID) == "" || at.IsZero() {
		return Repository{}, ErrState
	}
	input.Name = strings.TrimSpace(input.Name)
	input.DefaultBranch = strings.TrimSpace(input.DefaultBranch)
	if !validRepositoryID(input.ID) || input.Name == "" || len(input.Name) > 120 {
		return Repository{}, ErrInvalidPath
	}
	if input.DefaultBranch == "" {
		input.DefaultBranch = "main"
	}
	if err := validateBranch(input.DefaultBranch); err != nil {
		return Repository{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.repositories[input.ID]; exists {
		return Repository{}, ErrExists
	}
	target := filepath.Join(s.catalog.root, input.ID+".git")
	if filepath.Dir(target) != s.catalog.root {
		return Repository{}, ErrInvalidPath
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Repository{}, ErrExists
		}
		return Repository{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(target)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	command := exec.CommandContext(ctx, "git", "init", "--bare", "--initial-branch="+input.DefaultBranch, target)
	output, err := command.CombinedOutput()
	cancel()
	if err != nil {
		return Repository{}, fmt.Errorf("initialize repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := configureHostedRepository(context.Background(), target); err != nil {
		return Repository{}, err
	}
	repository := Repository{
		ID: input.ID, Name: input.Name, DefaultBranch: input.DefaultBranch, Path: target,
		Hosted: true, CreatedAt: at.UTC(),
	}
	if err := s.persist(repository); err != nil {
		return Repository{}, err
	}
	if err := s.catalog.audit.Append(audit.Entry{
		Actor: actor, Action: "repository.create", Target: "repository:" + repository.ID,
		RequestID: requestID, At: at.UTC(),
	}); err != nil {
		rollbackErr := s.catalog.store.DeleteMany(catalogNamespace, []string{repository.ID})
		if rollbackErr != nil {
			cleanup = false
		}
		return Repository{}, errors.Join(err, rollbackErr)
	}
	s.repositories[repository.ID] = repository
	cleanup = false
	return repository, nil
}

func configureHostedRepository(parent context.Context, target string) error {
	settings := [][2]string{
		{"uploadpack.allowFilter", "true"},
		{"uploadpack.allowAnySHA1InWant", "true"},
		{"uploadpack.allowReachableSHA1InWant", "true"},
		{"repack.writeBitmaps", "true"},
		{"pack.writeBitmapHashCache", "true"},
		{"gc.writeCommitGraph", "true"},
		{"fetch.writeCommitGraph", "true"},
	}
	for _, setting := range settings {
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		output, err := exec.CommandContext(ctx, "git", "-C", target, "config", setting[0], setting[1]).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("configure repository optimization %s: %w: %s", setting[0], err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func (s *Service) persist(repository Repository) error {
	record := catalogRecord{
		ID: repository.ID, SourceID: repository.SourceID, Name: repository.Name, DefaultBranch: repository.DefaultBranch,
		Path: repository.Path, Hosted: repository.Hosted, CreatedAt: repository.CreatedAt,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := s.catalog.store.Put(catalogNamespace, repository.ID, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func (record catalogRecord) repository() Repository {
	return Repository{
		ID: record.ID, SourceID: record.SourceID, Name: record.Name, DefaultBranch: record.DefaultBranch, Path: record.Path,
		Hosted: record.Hosted, CreatedAt: record.CreatedAt,
	}
}

func secureRepositoryRoot(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(root, 0o700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: repository root must be a directory", ErrInvalidPath)
	}
	return os.Chmod(root, 0o700)
}

func validateBranch(branch string) error {
	if len(branch) > 255 || strings.ContainsAny(branch, "\r\n\x00") {
		return ErrInvalidRef
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch).Run(); err != nil {
		return ErrInvalidRef
	}
	return nil
}
