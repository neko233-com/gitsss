// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

var releaseTargets = []struct {
	os   string
	arch string
}{
	{os: "darwin", arch: "amd64"},
	{os: "darwin", arch: "arm64"},
	{os: "linux", arch: "amd64"},
	{os: "linux", arch: "arm64"},
	{os: "windows", arch: "amd64"},
	{os: "windows", arch: "arm64"},
}

func buildManifest(releaseDirectory, version, commit, publishedAt string) (gitsssupdate.Manifest, error) {
	if !filepath.IsAbs(releaseDirectory) {
		return gitsssupdate.Manifest{}, errors.New("release directory must use an absolute path")
	}
	info, err := os.Stat(releaseDirectory)
	if err != nil || !info.IsDir() {
		return gitsssupdate.Manifest{}, errors.New("release directory does not exist")
	}
	manifest := gitsssupdate.Manifest{
		Format: gitsssupdate.Format, Version: version, Commit: commit, PublishedAt: publishedAt,
		Artifacts: make([]gitsssupdate.Artifact, 0, len(releaseTargets)),
	}
	for _, target := range releaseTargets {
		suffix := ".tar.gz"
		if target.os == "windows" {
			suffix = ".zip"
		}
		name := fmt.Sprintf("gitsss-%s-%s-%s%s", version, target.os, target.arch, suffix)
		artifact, err := inspectReleaseArtifact(filepath.Join(releaseDirectory, name), name, target.os, target.arch)
		if err != nil {
			return gitsssupdate.Manifest{}, err
		}
		manifest.Artifacts = append(manifest.Artifacts, artifact)
	}
	if _, err := gitsssupdate.Canonical(manifest); err != nil {
		return gitsssupdate.Manifest{}, err
	}
	return manifest, nil
}

func inspectReleaseArtifact(filePath, name, targetOS, targetArch string) (gitsssupdate.Artifact, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return gitsssupdate.Artifact{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 16<<30 {
		return gitsssupdate.Artifact{}, fmt.Errorf("release artifact must be a regular file: %s", name)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return gitsssupdate.Artifact{}, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return gitsssupdate.Artifact{}, err
	}
	currentInfo, err := os.Lstat(filePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		return gitsssupdate.Artifact{}, fmt.Errorf("release artifact changed while opening: %s", name)
	}
	hasher := sha256.New()
	written, err := io.CopyN(hasher, file, info.Size())
	if err != nil || written != info.Size() {
		return gitsssupdate.Artifact{}, fmt.Errorf("release artifact changed while hashing: %s", name)
	}
	var extra [1]byte
	if count, readErr := file.Read(extra[:]); count != 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		return gitsssupdate.Artifact{}, fmt.Errorf("release artifact grew while hashing: %s", name)
	}
	return gitsssupdate.Artifact{
		Name: name, OS: targetOS, Arch: targetArch, Size: info.Size(), SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func writeSignedManifest(releaseDirectory string, manifestContent, signatureContent, publicKeyContent []byte) error {
	files := []struct {
		name    string
		content []byte
	}{
		{name: "update-manifest.json", content: manifestContent},
		{name: "update-manifest.sig", content: signatureContent},
		{name: "release.pub", content: publicKeyContent},
	}
	created := make([]string, 0, len(files))
	for _, output := range files {
		filePath := filepath.Join(releaseDirectory, output.name)
		if err := atomicReleaseFile(filePath, output.content, 0o644); err != nil {
			for index := len(created) - 1; index >= 0; index-- {
				_ = os.Remove(created[index])
			}
			return err
		}
		created = append(created, filePath)
	}
	return nil
}

func atomicReleaseFile(filePath string, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(filePath)
	temporary, err := os.CreateTemp(directory, ".gitsss-release-partial-*")
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
	if err := temporary.Chmod(mode); err != nil {
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
