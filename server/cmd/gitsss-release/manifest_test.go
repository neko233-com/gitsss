// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

func TestBuildAndWriteSignedManifest(t *testing.T) {
	releaseDirectory := t.TempDir()
	version := "1.2.3-alpha.1"
	for index, target := range releaseTargets {
		suffix := ".tar.gz"
		if target.os == "windows" {
			suffix = ".zip"
		}
		name := "gitsss-" + version + "-" + target.os + "-" + target.arch + suffix
		if err := os.WriteFile(filepath.Join(releaseDirectory, name), []byte{byte(index + 1)}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := buildManifest(releaseDirectory, version, "0123456789abcdef0123456789abcdef01234567", "2026-07-14T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Artifacts) != 6 {
		t.Fatalf("artifacts = %d", len(manifest.Artifacts))
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestContent, signatureContent, err := gitsssupdate.Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeSignedManifest(releaseDirectory, manifestContent, signatureContent, gitsssupdate.EncodePublicKey(publicKey)); err != nil {
		t.Fatal(err)
	}
	storedManifest, err := os.ReadFile(filepath.Join(releaseDirectory, "update-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	storedSignature, err := os.ReadFile(filepath.Join(releaseDirectory, "update-manifest.sig"))
	if err != nil {
		t.Fatal(err)
	}
	verified, _, err := gitsssupdate.Verify(storedManifest, storedSignature, gitsssupdate.EncodePublicKey(publicKey))
	if err != nil || verified.Version != version {
		t.Fatalf("verified = %#v, %v", verified, err)
	}
	if err := writeSignedManifest(releaseDirectory, manifestContent, signatureContent, gitsssupdate.EncodePublicKey(publicKey)); err == nil {
		t.Fatal("existing signed manifest was overwritten")
	}
}

func TestBuildManifestRejectsSymlinkArtifact(t *testing.T) {
	releaseDirectory := t.TempDir()
	version := "1.2.3"
	var firstPath string
	for index, target := range releaseTargets {
		suffix := ".tar.gz"
		if target.os == "windows" {
			suffix = ".zip"
		}
		filePath := filepath.Join(releaseDirectory, "gitsss-"+version+"-"+target.os+"-"+target.arch+suffix)
		if index == 0 {
			firstPath = filePath
			continue
		}
		if err := os.WriteFile(filePath, []byte{byte(index)}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	targetPath := filepath.Join(releaseDirectory, "target")
	if err := os.WriteFile(targetPath, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, firstPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := buildManifest(releaseDirectory, version, "0123456789abcdef0123456789abcdef01234567", "2026-07-14T12:00:00Z")
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink error = %v", err)
	}
}
