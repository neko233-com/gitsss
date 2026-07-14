// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

func TestVerifyUpdateFilesUsesTrustedKeyAndExactPackage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	packageName := "gitsss-1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH + updateTestArchiveSuffix()
	packagePath := filepath.Join(root, packageName)
	packageContent := []byte("verified package")
	if err := os.WriteFile(packagePath, packageContent, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(packageContent)
	manifestContent, signatureContent, err := gitsssupdate.Sign(gitsssupdate.Manifest{
		Format: gitsssupdate.Format, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		PublishedAt: "2026-07-14T12:00:00Z",
		Artifacts: []gitsssupdate.Artifact{{
			Name: packageName, OS: runtime.GOOS, Arch: runtime.GOARCH,
			Size: int64(len(packageContent)), SHA256: hex.EncodeToString(digest[:]),
		}},
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := writeUpdateTestFile(t, root, "update-manifest.json", manifestContent)
	signaturePath := writeUpdateTestFile(t, root, "update-manifest.sig", signatureContent)
	publicKeyPath := writeUpdateTestFile(t, root, "release.pub", gitsssupdate.EncodePublicKey(publicKey))
	result, err := verifyUpdateFiles(manifestPath, signaturePath, publicKeyPath, packagePath, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "1.2.3" || result.KeyID != gitsssupdate.KeyID(publicKey) || result.Artifact.Name != packageName {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := verifyUpdateFiles(manifestPath, signaturePath, publicKeyPath, packagePath, runtime.GOOS, "unsupported"); err == nil {
		t.Fatal("wrong target architecture was accepted")
	}
}

func writeUpdateTestFile(t *testing.T, root, name string, content []byte) string {
	t.Helper()
	filePath := filepath.Join(root, name)
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return filePath
}

func updateTestArchiveSuffix() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}
