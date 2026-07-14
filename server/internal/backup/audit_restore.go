// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxAuditRepositories = 10_000
	gitAuditTimeout      = 2 * time.Minute
	maxGitErrorBytes     = 64 << 10
)

type GitAudit struct {
	Path            string `json:"path"`
	Valid           bool   `json:"valid"`
	Objects         int64  `json:"objects"`
	Refs            int64  `json:"refs"`
	ReferenceSHA256 string `json:"reference_sha256,omitempty"`
	Error           string `json:"error,omitempty"`
}

type LFSAuditIssue struct {
	Path     string `json:"path"`
	Expected string `json:"expected_oid"`
	Actual   string `json:"actual_sha256"`
}

type LFSAudit struct {
	Objects int             `json:"objects"`
	Invalid []LFSAuditIssue `json:"invalid,omitempty"`
}

type RestoreAudit struct {
	AssessmentID       string        `json:"assessment_id"`
	Target             string        `json:"target"`
	Files              ImportSummary `json:"files"`
	Git                []GitAudit    `json:"git"`
	LFS                LFSAudit      `json:"lfs"`
	Findings           []string      `json:"findings,omitempty"`
	ReadyForShadowMode bool          `json:"ready_for_shadow_mode"`
	AuditedAt          time.Time     `json:"audited_at"`
}

func AuditRestore(inputPath, passphrase, target string) (RestoreAudit, error) {
	verification, err := Verify(inputPath, passphrase)
	if err != nil {
		return RestoreAudit{}, err
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return RestoreAudit{}, err
	}
	contentRoot := filepath.Join(absolute, "content")
	plan, err := BuildImportPlan(verification, contentRoot, time.Now())
	if err != nil {
		return RestoreAudit{}, err
	}
	audit := RestoreAudit{
		AssessmentID: verification.Manifest.Source.AssessmentID,
		Target:       absolute, Files: plan.Summary, LFS: auditLFS(verification.Manifest),
		AuditedAt: time.Now().UTC(),
	}
	repositories, unclassified := repositoryRoots(verification.Manifest)
	if unclassified > 0 {
		audit.Findings = append(audit.Findings, fmt.Sprintf("%d repository files were not inside a .git directory", unclassified))
	}
	if len(repositories) > maxAuditRepositories {
		return RestoreAudit{}, fmt.Errorf("%w: restore contains more than %d repositories", ErrInvalidBackup, maxAuditRepositories)
	}
	gitExecutable, gitErr := exec.LookPath("git")
	for _, repository := range repositories {
		if gitErr != nil {
			audit.Git = append(audit.Git, GitAudit{Path: repository, Error: "git executable is unavailable"})
			continue
		}
		audit.Git = append(audit.Git, auditGitRepository(gitExecutable, contentRoot, repository))
	}
	filesReady := audit.Files.New == 0 && audit.Files.Conflicts == 0 && audit.Files.Blocked == 0 && audit.Files.Identical == len(verification.Manifest.Entries)
	gitReady := unclassified == 0
	for _, repository := range audit.Git {
		gitReady = gitReady && repository.Valid
	}
	audit.ReadyForShadowMode = filesReady && gitReady && len(audit.LFS.Invalid) == 0
	return audit, nil
}

func repositoryRoots(manifest Manifest) ([]string, int) {
	seen := map[string]struct{}{}
	unclassified := 0
	for _, entry := range manifest.Entries {
		if entry.Kind != "repositories" {
			continue
		}
		components := strings.Split(entry.Path, "/")
		found := ""
		for index, component := range components {
			if strings.HasSuffix(strings.ToLower(component), ".git") {
				found = path.Join(components[:index+1]...)
				break
			}
		}
		if found == "" {
			unclassified++
			continue
		}
		seen[found] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for repository := range seen {
		result = append(result, repository)
	}
	sort.Strings(result)
	return result, unclassified
}

func auditGitRepository(gitExecutable, contentRoot, relative string) GitAudit {
	audit := GitAudit{Path: relative}
	repositoryPath := filepath.Join(contentRoot, filepath.FromSlash(relative))
	if reason := inspectImportAncestors(repositoryPath); reason != "" {
		audit.Error = reason
		return audit
	}
	info, err := os.Stat(repositoryPath)
	if err != nil || !info.IsDir() {
		audit.Error = "repository directory is missing"
		return audit
	}
	if alternates, err := os.ReadFile(filepath.Join(repositoryPath, "objects", "info", "alternates")); err == nil && strings.TrimSpace(string(alternates)) != "" {
		audit.Error = "repository uses object alternates outside the verified manifest"
		return audit
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		audit.Error = "repository object alternates cannot be inspected"
		return audit
	}
	context, cancel := context.WithTimeout(context.Background(), gitAuditTimeout)
	defer cancel()
	if output, err := runGit(context, gitExecutable, repositoryPath, "fsck", "--full", "--strict"); err != nil {
		audit.Error = gitError(context, err, output)
		return audit
	}
	objectsOutput, err := runGit(context, gitExecutable, repositoryPath, "count-objects", "-v")
	if err != nil {
		audit.Error = gitError(context, err, objectsOutput)
		return audit
	}
	audit.Objects, err = parseObjectCount(objectsOutput)
	if err != nil {
		audit.Error = err.Error()
		return audit
	}
	audit.Refs, audit.ReferenceSHA256, err = referenceDigest(context, gitExecutable, repositoryPath)
	if err != nil {
		audit.Error = err.Error()
		return audit
	}
	audit.Valid = true
	return audit
}

func runGit(ctx context.Context, executable, repository string, arguments ...string) (string, error) {
	commandArguments := append([]string{"-C", repository}, arguments...)
	command := exec.CommandContext(ctx, executable, commandArguments...)
	output := &limitedBuffer{limit: maxGitErrorBytes}
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	return strings.TrimSpace(output.String()), err
}

func referenceDigest(ctx context.Context, executable, repository string) (int64, string, error) {
	command := exec.CommandContext(ctx, executable, "-C", repository, "show-ref", "--head", "-d")
	output := &lineHashWriter{hash: sha256.New()}
	stderr := &limitedBuffer{limit: maxGitErrorBytes}
	command.Stdout = output
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 || output.bytes != 0 {
			return 0, "", fmt.Errorf("git show-ref failed: %s", gitError(ctx, err, stderr.String()))
		}
	}
	return output.lines, hex.EncodeToString(output.hash.Sum(nil)), nil
}

func parseObjectCount(output string) (int64, error) {
	values := map[string]int64{}
	for _, line := range strings.Split(output, "\n") {
		key, raw, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			values[key] = value
		}
	}
	count, countOK := values["count"]
	inPack, packOK := values["in-pack"]
	if !countOK || !packOK {
		return 0, errors.New("git count-objects returned an incomplete result")
	}
	return count + inPack, nil
}

func auditLFS(manifest Manifest) LFSAudit {
	audit := LFSAudit{}
	for _, entry := range manifest.Entries {
		if entry.Kind != "lfs" {
			continue
		}
		oid := path.Base(entry.Path)
		if len(oid) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(oid); err != nil {
			continue
		}
		audit.Objects++
		if !strings.EqualFold(oid, entry.SHA256) {
			audit.Invalid = append(audit.Invalid, LFSAuditIssue{Path: entry.Path, Expected: strings.ToLower(oid), Actual: entry.SHA256})
		}
	}
	return audit
}

func gitError(ctx context.Context, err error, output string) string {
	if ctx.Err() != nil {
		return "git verification timed out"
	}
	if output != "" {
		return output
	}
	return err.Error()
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *limitedBuffer) Write(content []byte) (int, error) {
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if remaining > len(content) {
			remaining = len(content)
		}
		_, _ = buffer.buffer.Write(content[:remaining])
	}
	return len(content), nil
}

func (buffer *limitedBuffer) String() string {
	return buffer.buffer.String()
}

type lineHashWriter struct {
	hash  hash.Hash
	lines int64
	bytes int64
}

func (writer *lineHashWriter) Write(content []byte) (int, error) {
	written, err := writer.hash.Write(content)
	writer.lines += int64(bytes.Count(content, []byte{'\n'}))
	writer.bytes += int64(written)
	return written, err
}
