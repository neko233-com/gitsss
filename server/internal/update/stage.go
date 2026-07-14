// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxStageEntries = 20_000
	maxStageFile    = 512 << 20
	maxStageBytes   = 4 << 30
	stageReadyName  = ".gitsss-update-stage.json"
)

var ErrInvalidArchive = errors.New("invalid update archive")

type StageResult struct {
	Version     string   `json:"version"`
	Commit      string   `json:"commit"`
	PublishedAt string   `json:"published_at"`
	KeyID       string   `json:"key_id"`
	Artifact    Artifact `json:"artifact"`
	Target      string   `json:"target"`
	Files       int      `json:"files"`
	Bytes       int64    `json:"bytes"`
	StagedAt    string   `json:"staged_at"`
}

type stageWriter struct {
	root        string
	archiveRoot string
	seen        map[string]stagePath
	files       int
	bytes       int64
}

type stagePath struct {
	original  string
	directory bool
	explicit  bool
}

func VerifyAndStage(manifestContent, signatureContent, publicKeyContent []byte, packagePath, targetOS, targetArch, targetPath string, now time.Time) (StageResult, error) {
	manifest, signature, err := Verify(manifestContent, signatureContent, publicKeyContent)
	if err != nil {
		return StageResult{}, err
	}
	artifact, found := artifactForTarget(manifest, targetOS, targetArch)
	if !found {
		return StageResult{}, fmt.Errorf("%w: no artifact for %s/%s", ErrArtifactMismatch, targetOS, targetArch)
	}
	if !filepath.IsAbs(packagePath) || !filepath.IsAbs(targetPath) || filepath.Clean(targetPath) != targetPath {
		return StageResult{}, fmt.Errorf("%w: package and staging paths must be clean absolute paths", ErrInvalidArchive)
	}
	if filepath.Base(packagePath) != artifact.Name {
		return StageResult{}, fmt.Errorf("%w: filename differs", ErrArtifactMismatch)
	}
	packageFile, err := openVerifiedArtifact(packagePath, artifact)
	if err != nil {
		return StageResult{}, err
	}
	defer packageFile.Close()
	if err := os.Mkdir(targetPath, 0o700); err != nil {
		return StageResult{}, fmt.Errorf("create staging directory: %w", err)
	}
	completed := false
	defer func() {
		if !completed {
			_ = os.RemoveAll(targetPath)
		}
	}()
	writer := &stageWriter{
		root: targetPath, archiveRoot: archiveRootName(artifact.Name),
		seen: make(map[string]stagePath),
	}
	if artifact.OS == "windows" {
		err = extractZIP(packageFile, artifact.Size, writer)
	} else {
		err = extractTarGZIP(packageFile, writer)
	}
	if err != nil {
		return StageResult{}, err
	}
	result := StageResult{
		Version: manifest.Version, Commit: manifest.Commit, PublishedAt: manifest.PublishedAt,
		KeyID: signature.KeyID, Artifact: artifact, Target: targetPath,
		Files: writer.files, Bytes: writer.bytes, StagedAt: now.UTC().Format(time.RFC3339),
	}
	if result.Files == 0 || result.StagedAt == "" {
		return StageResult{}, fmt.Errorf("%w: archive is empty or staging time is invalid", ErrInvalidArchive)
	}
	metadata, err := json.Marshal(result)
	if err != nil {
		return StageResult{}, err
	}
	metadata = append(metadata, '\n')
	if err := writeStageFile(filepath.Join(targetPath, stageReadyName), metadata, 0o600); err != nil {
		return StageResult{}, err
	}
	if runtime.GOOS != "windows" {
		directory, err := os.Open(targetPath)
		if err != nil {
			return StageResult{}, err
		}
		if err := directory.Sync(); err != nil {
			directory.Close()
			return StageResult{}, err
		}
		if err := directory.Close(); err != nil {
			return StageResult{}, err
		}
	}
	completed = true
	return result, nil
}

func artifactForTarget(manifest Manifest, targetOS, targetArch string) (Artifact, bool) {
	for _, artifact := range manifest.Artifacts {
		if artifact.OS == targetOS && artifact.Arch == targetArch {
			return artifact, true
		}
	}
	return Artifact{}, false
}

func openVerifiedArtifact(packagePath string, artifact Artifact) (*os.File, error) {
	info, err := os.Lstat(packagePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != artifact.Size {
		return nil, fmt.Errorf("%w: file type or size differs", ErrArtifactMismatch)
	}
	file, err := os.Open(packagePath)
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	currentInfo, err := os.Lstat(packagePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		file.Close()
		return nil, fmt.Errorf("%w: file changed while opening", ErrArtifactMismatch)
	}
	hasher := sha256.New()
	written, err := io.CopyN(hasher, file, artifact.Size)
	if err != nil || written != artifact.Size || hex.EncodeToString(hasher.Sum(nil)) != artifact.SHA256 {
		file.Close()
		return nil, fmt.Errorf("%w: SHA-256 differs", ErrArtifactMismatch)
	}
	var extra [1]byte
	if count, readErr := file.Read(extra[:]); count != 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		file.Close()
		return nil, fmt.Errorf("%w: package grew while verifying", ErrArtifactMismatch)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func extractZIP(packageFile *os.File, size int64, writer *stageWriter) error {
	archive, err := zip.NewReader(packageFile, size)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(archive.File) > maxStageEntries {
		return fmt.Errorf("%w: too many entries", ErrInvalidArchive)
	}
	for _, entry := range archive.File {
		mode := entry.Mode()
		if mode&os.ModeType != 0 && !mode.IsDir() {
			return fmt.Errorf("%w: unsupported ZIP entry %s", ErrInvalidArchive, entry.Name)
		}
		if entry.UncompressedSize64 > maxStageFile {
			return fmt.Errorf("%w: entry is too large", ErrInvalidArchive)
		}
		if mode.IsDir() || strings.HasSuffix(strings.ReplaceAll(entry.Name, "\\", "/"), "/") {
			if err := writer.directory(entry.Name); err != nil {
				return err
			}
			continue
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		err = writer.file(entry.Name, int64(entry.UncompressedSize64), source)
		closeErr := source.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTarGZIP(packageFile *os.File, writer *stageWriter) error {
	compressed, err := gzip.NewReader(packageFile)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	archive := tar.NewReader(compressed)
	entries := 0
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = compressed.Close()
			return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
		}
		entries++
		if entries > maxStageEntries {
			_ = compressed.Close()
			return fmt.Errorf("%w: too many entries", ErrInvalidArchive)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := writer.directory(header.Name); err != nil {
				_ = compressed.Close()
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxStageFile {
				_ = compressed.Close()
				return fmt.Errorf("%w: entry is too large", ErrInvalidArchive)
			}
			if err := writer.file(header.Name, header.Size, archive); err != nil {
				_ = compressed.Close()
				return err
			}
		default:
			_ = compressed.Close()
			return fmt.Errorf("%w: unsupported TAR entry %s", ErrInvalidArchive, header.Name)
		}
	}
	return compressed.Close()
}

func (writer *stageWriter) directory(name string) error {
	relative, key, rootOnly, err := writer.validateName(name)
	if err != nil {
		return err
	}
	if rootOnly {
		return nil
	}
	if err := writer.claimParents(relative); err != nil {
		return err
	}
	if existing, exists := writer.seen[key]; exists {
		if !existing.directory || existing.original != relative || existing.explicit {
			return fmt.Errorf("%w: duplicate or case-conflicting path %s", ErrInvalidArchive, name)
		}
		existing.explicit = true
		writer.seen[key] = existing
	} else {
		writer.seen[key] = stagePath{original: relative, directory: true, explicit: true}
	}
	return os.MkdirAll(filepath.Join(writer.root, filepath.FromSlash(relative)), 0o755)
}

func (writer *stageWriter) file(name string, size int64, source io.Reader) error {
	relative, key, rootOnly, err := writer.validateName(name)
	if err != nil {
		return err
	}
	if rootOnly || size < 0 || size > maxStageFile || writer.bytes > maxStageBytes-size {
		return fmt.Errorf("%w: invalid file entry %s", ErrInvalidArchive, name)
	}
	if err := writer.claimParents(relative); err != nil {
		return err
	}
	if _, exists := writer.seen[key]; exists {
		return fmt.Errorf("%w: duplicate or case-conflicting path %s", ErrInvalidArchive, name)
	}
	writer.seen[key] = stagePath{original: relative}
	destination := filepath.Join(writer.root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if path.Base(relative) == "gitsss" || path.Base(relative) == "gitsss-agent" {
		mode = 0o755
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(file, source, size)
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil || written != size {
		return fmt.Errorf("%w: truncated entry %s", ErrInvalidArchive, name)
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	writer.files++
	writer.bytes += size
	return nil
}

func (writer *stageWriter) claimParents(relative string) error {
	segments := strings.Split(relative, "/")
	for index := 1; index < len(segments); index++ {
		parent := strings.Join(segments[:index], "/")
		key := strings.ToLower(parent)
		if existing, exists := writer.seen[key]; exists {
			if !existing.directory || existing.original != parent {
				return fmt.Errorf("%w: case-conflicting parent %s", ErrInvalidArchive, parent)
			}
			continue
		}
		writer.seen[key] = stagePath{original: parent, directory: true}
	}
	return nil
}

func (writer *stageWriter) validateName(name string) (string, string, bool, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	cleaned := path.Clean(normalized)
	if normalized == "" || strings.HasPrefix(normalized, "/") || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.ContainsRune(cleaned, '\x00') {
		return "", "", false, fmt.Errorf("%w: unsafe path %s", ErrInvalidArchive, name)
	}
	if cleaned == writer.archiveRoot {
		return "", "", true, nil
	}
	prefix := writer.archiveRoot + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", "", false, fmt.Errorf("%w: entry is outside archive root", ErrInvalidArchive)
	}
	relative := strings.TrimPrefix(cleaned, prefix)
	for _, segment := range strings.Split(relative, "/") {
		if !safeArchiveSegment(segment) {
			return "", "", false, fmt.Errorf("%w: unsafe path segment %s", ErrInvalidArchive, segment)
		}
	}
	return relative, strings.ToLower(relative), false, nil
}

func safeArchiveSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "\x00:") || strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
		return false
	}
	base := strings.ToUpper(strings.TrimSuffix(segment, path.Ext(segment)))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}
	return true
}

func archiveRootName(name string) string {
	if strings.HasSuffix(name, ".tar.gz") {
		return strings.TrimSuffix(name, ".tar.gz")
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func writeStageFile(filePath string, content []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
