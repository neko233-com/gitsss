// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyAndStageZIPPublishesReadyMetadata(t *testing.T) {
	root := t.TempDir()
	version := "1.2.3"
	name := "gitsss-" + version + "-windows-amd64.zip"
	packagePath := filepath.Join(root, name)
	writeTestZIP(t, packagePath, map[string]string{
		"gitsss-1.2.3-windows-amd64/gitsss.exe":       "server",
		"gitsss-1.2.3-windows-amd64/gitsss-agent.exe": "agent",
		"gitsss-1.2.3-windows-amd64/web/index.html":   "web",
	})
	manifestContent, signatureContent, publicKey := signedStageTestManifest(t, packagePath, name, "windows", "amd64", version)
	target := filepath.Join(root, "staged")
	result, err := VerifyAndStage(manifestContent, signatureContent, publicKey, packagePath, "windows", "amd64", target, time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 3 || result.Bytes != int64(len("serveragentweb")) || result.StagedAt != "2026-07-14T12:00:00Z" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, relative := range []string{"gitsss.exe", "gitsss-agent.exe", filepath.Join("web", "index.html"), stageReadyName} {
		if _, err := os.Stat(filepath.Join(target, relative)); err != nil {
			t.Fatalf("missing %s: %v", relative, err)
		}
	}
}

func TestVerifyAndStageRejectsTraversalAndCleansTarget(t *testing.T) {
	root := t.TempDir()
	name := "gitsss-1.2.3-windows-amd64.zip"
	packagePath := filepath.Join(root, name)
	writeTestZIP(t, packagePath, map[string]string{"gitsss-1.2.3-windows-amd64/../../escape": "bad"})
	manifestContent, signatureContent, publicKey := signedStageTestManifest(t, packagePath, name, "windows", "amd64", "1.2.3")
	target := filepath.Join(root, "staged")
	if _, err := VerifyAndStage(manifestContent, signatureContent, publicKey, packagePath, "windows", "amd64", target, time.Now()); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failed staging target remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatalf("archive escaped target: %v", err)
	}
}

func TestVerifyAndStageRejectsDuplicateCaseAndTampering(t *testing.T) {
	root := t.TempDir()
	name := "gitsss-1.2.3-windows-amd64.zip"
	packagePath := filepath.Join(root, name)
	writeTestZIP(t, packagePath, map[string]string{
		"gitsss-1.2.3-windows-amd64/web/File.txt": "one",
		"gitsss-1.2.3-windows-amd64/web/file.txt": "two",
	})
	manifestContent, signatureContent, publicKey := signedStageTestManifest(t, packagePath, name, "windows", "amd64", "1.2.3")
	if _, err := VerifyAndStage(manifestContent, signatureContent, publicKey, packagePath, "windows", "amd64", filepath.Join(root, "case"), time.Now()); err == nil {
		t.Fatal("case-colliding paths were accepted")
	}
	packageContent, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	packageContent[len(packageContent)/2] ^= 0xff
	if err := os.WriteFile(packagePath, packageContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyAndStage(manifestContent, signatureContent, publicKey, packagePath, "windows", "amd64", filepath.Join(root, "tampered"), time.Now()); err == nil {
		t.Fatal("tampered package was accepted")
	}
}

func TestVerifyAndStageTarGZIPAndRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	name := "gitsss-1.2.3-linux-amd64.tar.gz"
	packagePath := filepath.Join(root, name)
	writeTestTarGZIP(t, packagePath, false)
	manifestContent, signatureContent, publicKey := signedStageTestManifest(t, packagePath, name, "linux", "amd64", "1.2.3")
	result, err := VerifyAndStage(manifestContent, signatureContent, publicKey, packagePath, "linux", "amd64", filepath.Join(root, "valid"), time.Now())
	if err != nil || result.Files != 2 {
		t.Fatalf("result = %#v, %v", result, err)
	}

	badPackage := filepath.Join(root, "bad", name)
	if err := os.MkdirAll(filepath.Dir(badPackage), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestTarGZIP(t, badPackage, true)
	badManifest, badSignature, badPublicKey := signedStageTestManifest(t, badPackage, name, "linux", "amd64", "1.2.3")
	badTarget := filepath.Join(root, "symlink")
	if _, err := VerifyAndStage(badManifest, badSignature, badPublicKey, badPackage, "linux", "amd64", badTarget, time.Now()); err == nil {
		t.Fatal("TAR symlink was accepted")
	}
	if _, err := os.Stat(badTarget); !os.IsNotExist(err) {
		t.Fatalf("failed TAR staging target remains: %v", err)
	}
}

func writeTestZIP(t *testing.T, filePath string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTestTarGZIP(t *testing.T, filePath string, includeSymlink bool) {
	t.Helper()
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	entries := map[string]string{
		"gitsss-1.2.3-linux-amd64/gitsss":         "server",
		"gitsss-1.2.3-linux-amd64/web/index.html": "web",
	}
	for name, content := range entries {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if includeSymlink {
		if err := archive.WriteHeader(&tar.Header{Name: "gitsss-1.2.3-linux-amd64/link", Linkname: "gitsss", Typeflag: tar.TypeSymlink}); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func signedStageTestManifest(t *testing.T, packagePath, name, targetOS, targetArch, version string) ([]byte, []byte, []byte) {
	t.Helper()
	content, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestContent, signatureContent, err := Sign(Manifest{
		Format: Format, Version: version, Commit: "0123456789abcdef0123456789abcdef01234567",
		PublishedAt: "2026-07-14T12:00:00Z",
		Artifacts: []Artifact{{
			Name: name, OS: targetOS, Arch: targetArch, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]),
		}},
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return manifestContent, signatureContent, EncodePublicKey(publicKey)
}

func TestStageWriterRejectsReservedWindowsName(t *testing.T) {
	writer := &stageWriter{root: t.TempDir(), archiveRoot: "root", seen: make(map[string]stagePath)}
	if err := writer.file("root/CON.txt", int64(len("bad")), bytes.NewBufferString("bad")); err == nil {
		t.Fatal("reserved Windows path was accepted")
	}
}

func TestStageWriterRejectsCaseConflictingParent(t *testing.T) {
	writer := &stageWriter{root: t.TempDir(), archiveRoot: "root", seen: make(map[string]stagePath)}
	if err := writer.file("root/Web/one.txt", 3, bytes.NewBufferString("one")); err != nil {
		t.Fatal(err)
	}
	if err := writer.file("root/web/two.txt", 3, bytes.NewBufferString("two")); err == nil {
		t.Fatal("case-conflicting parent was accepted")
	}
}
