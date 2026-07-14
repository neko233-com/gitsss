// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSignVerifyAndArtifactIntegrity(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("signed update package")
	digest := sha256.Sum256(content)
	name := "gitsss-1.2.3-" + runtime.GOOS + "-" + runtime.GOARCH + archiveSuffix(runtime.GOOS)
	manifest := Manifest{
		Format: Format, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		PublishedAt: "2026-07-14T12:00:00Z",
		Artifacts:   []Artifact{{Name: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}},
	}
	manifestContent, signatureContent, err := Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, signature, err := Verify(manifestContent, signatureContent, EncodePublicKey(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	if signature.KeyID != KeyID(publicKey) || verified.Version != manifest.Version {
		t.Fatalf("unexpected verification: %#v %#v", verified, signature)
	}
	packagePath := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(packagePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact, err := VerifyArtifact(verified, packagePath, runtime.GOOS, runtime.GOARCH)
	if err != nil || artifact.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("artifact = %#v, %v", artifact, err)
	}
	content[0] ^= 0xff
	if err := os.WriteFile(packagePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArtifact(verified, packagePath, runtime.GOOS, runtime.GOARCH); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("tampered artifact error = %v", err)
	}
}

func TestVerifyRejectsTamperingWrongKeyAndNoncanonicalJSON(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := validTestManifest()
	manifestContent, signatureContent, err := Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), manifestContent...)
	tampered[20] ^= 1
	if _, _, err := Verify(tampered, signatureContent, EncodePublicKey(publicKey)); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
	wrongPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Verify(manifestContent, signatureContent, EncodePublicKey(wrongPublic)); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong key error = %v", err)
	}
	if _, _, err := Verify(manifestContent, append(signatureContent, '\n'), EncodePublicKey(publicKey)); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("noncanonical signature error = %v", err)
	}
	pretty, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	prettySignature := Signature{Algorithm: "ed25519", KeyID: KeyID(publicKey), Value: "invalid"}
	prettySignatureContent, _ := json.Marshal(prettySignature)
	if _, _, err := Verify(pretty, prettySignatureContent, EncodePublicKey(publicKey)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("noncanonical manifest error = %v", err)
	}
}

func TestCanonicalRejectsDuplicateTargetsAndUnsafeNames(t *testing.T) {
	manifest := validTestManifest()
	manifest.Artifacts = append(manifest.Artifacts, manifest.Artifacts[0])
	manifest.Artifacts[1].Name = "second.zip"
	if _, err := Canonical(manifest); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("duplicate target error = %v", err)
	}
	manifest = validTestManifest()
	manifest.Artifacts[0].Name = "../package.zip"
	if _, err := Canonical(manifest); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("unsafe name error = %v", err)
	}
}

func validTestManifest() Manifest {
	return Manifest{
		Format: Format, Version: "1.2.3", Commit: "0123456789abcdef0123456789abcdef01234567",
		PublishedAt: "2026-07-14T12:00:00Z",
		Artifacts: []Artifact{{
			Name: "gitsss-1.2.3-linux-amd64.tar.gz", OS: "linux", Arch: "amd64", Size: 12,
			SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}},
	}
}

func archiveSuffix(targetOS string) string {
	if targetOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}
