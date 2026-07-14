// Copyright 2026 gitsss Authors. All rights reserved.

package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	Format           = "gitsss-update-v1"
	maxManifestSize  = 1 << 20
	maxSignatureSize = 4 << 10
	maxKeySize       = 1 << 10
)

var (
	ErrInvalidManifest  = errors.New("invalid update manifest")
	ErrInvalidSignature = errors.New("invalid update signature")
	ErrArtifactMismatch = errors.New("update artifact mismatch")
)

type Manifest struct {
	Format      string     `json:"format"`
	Version     string     `json:"version"`
	Commit      string     `json:"commit"`
	PublishedAt string     `json:"published_at"`
	Artifacts   []Artifact `json:"artifacts"`
}

type Artifact struct {
	Name   string `json:"name"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"`
}

func Canonical(manifest Manifest) ([]byte, error) {
	manifest.Artifacts = append([]Artifact(nil), manifest.Artifacts...)
	sort.Slice(manifest.Artifacts, func(left, right int) bool {
		if manifest.Artifacts[left].OS != manifest.Artifacts[right].OS {
			return manifest.Artifacts[left].OS < manifest.Artifacts[right].OS
		}
		if manifest.Artifacts[left].Arch != manifest.Artifacts[right].Arch {
			return manifest.Artifacts[left].Arch < manifest.Artifacts[right].Arch
		}
		return manifest.Artifacts[left].Name < manifest.Artifacts[right].Name
	})
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func Sign(manifest Manifest, privateKey ed25519.PrivateKey) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("%w: Ed25519 private key is required", ErrInvalidSignature)
	}
	content, err := Canonical(manifest)
	if err != nil {
		return nil, nil, err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	signature := Signature{
		Algorithm: "ed25519", KeyID: KeyID(publicKey),
		Value: base64.RawStdEncoding.EncodeToString(ed25519.Sign(privateKey, content)),
	}
	signatureContent, err := json.Marshal(signature)
	if err != nil {
		return nil, nil, err
	}
	return content, append(signatureContent, '\n'), nil
}

func Verify(manifestContent, signatureContent, publicKeyContent []byte) (Manifest, Signature, error) {
	if len(manifestContent) == 0 || len(manifestContent) > maxManifestSize || len(signatureContent) == 0 || len(signatureContent) > maxSignatureSize || len(publicKeyContent) == 0 || len(publicKeyContent) > maxKeySize {
		return Manifest{}, Signature{}, fmt.Errorf("%w: input size is invalid", ErrInvalidSignature)
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestContent))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, Signature{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	canonical, err := Canonical(manifest)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	if !bytes.Equal(manifestContent, canonical) {
		return Manifest{}, Signature{}, fmt.Errorf("%w: JSON is not canonical", ErrInvalidManifest)
	}
	var signature Signature
	decoder = json.NewDecoder(bytes.NewReader(signatureContent))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&signature); err != nil || signature.Algorithm != "ed25519" {
		return Manifest{}, Signature{}, fmt.Errorf("%w: malformed signature envelope", ErrInvalidSignature)
	}
	canonicalSignature, err := json.Marshal(signature)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	canonicalSignature = append(canonicalSignature, '\n')
	if !bytes.Equal(signatureContent, canonicalSignature) {
		return Manifest{}, Signature{}, fmt.Errorf("%w: signature envelope is not canonical", ErrInvalidSignature)
	}
	publicKey, err := DecodePublicKey(publicKeyContent)
	if err != nil {
		return Manifest{}, Signature{}, err
	}
	value, err := base64.RawStdEncoding.DecodeString(signature.Value)
	if err != nil || len(value) != ed25519.SignatureSize || signature.KeyID != KeyID(publicKey) || !ed25519.Verify(publicKey, manifestContent, value) {
		return Manifest{}, Signature{}, fmt.Errorf("%w: verification failed", ErrInvalidSignature)
	}
	return manifest, signature, nil
}

func VerifyArtifact(manifest Manifest, packagePath, targetOS, targetArch string) (Artifact, error) {
	if _, err := Canonical(manifest); err != nil {
		return Artifact{}, err
	}
	var expected *Artifact
	for index := range manifest.Artifacts {
		artifact := &manifest.Artifacts[index]
		if artifact.OS == targetOS && artifact.Arch == targetArch {
			expected = artifact
			break
		}
	}
	if expected == nil {
		return Artifact{}, fmt.Errorf("%w: no artifact for %s/%s", ErrArtifactMismatch, targetOS, targetArch)
	}
	if filepath.Base(packagePath) != expected.Name {
		return Artifact{}, fmt.Errorf("%w: filename differs", ErrArtifactMismatch)
	}
	info, err := os.Lstat(packagePath)
	if err != nil {
		return Artifact{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != expected.Size {
		return Artifact{}, fmt.Errorf("%w: file type or size differs", ErrArtifactMismatch)
	}
	file, err := os.Open(packagePath)
	if err != nil {
		return Artifact{}, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return Artifact{}, err
	}
	currentInfo, err := os.Lstat(packagePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		return Artifact{}, fmt.Errorf("%w: file changed while opening", ErrArtifactMismatch)
	}
	hasher := sha256.New()
	written, err := io.CopyN(hasher, file, expected.Size)
	if err != nil || written != expected.Size {
		return Artifact{}, fmt.Errorf("%w: file content is incomplete", ErrArtifactMismatch)
	}
	var extra [1]byte
	if count, readErr := file.Read(extra[:]); count != 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		return Artifact{}, fmt.Errorf("%w: file content grew", ErrArtifactMismatch)
	}
	if hex.EncodeToString(hasher.Sum(nil)) != expected.SHA256 {
		return Artifact{}, fmt.Errorf("%w: SHA-256 differs", ErrArtifactMismatch)
	}
	return *expected, nil
}

func EncodePublicKey(publicKey ed25519.PublicKey) []byte {
	return []byte(base64.RawStdEncoding.EncodeToString(publicKey) + "\n")
}

func EncodePrivateSeed(privateKey ed25519.PrivateKey) string {
	if len(privateKey) != ed25519.PrivateKeySize {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(privateKey.Seed())
}

func DecodePrivateSeed(value string) (ed25519.PrivateKey, error) {
	seed, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%w: private seed is malformed", ErrInvalidSignature)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func DecodePublicKey(content []byte) (ed25519.PublicKey, error) {
	decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(content)))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: public key is malformed", ErrInvalidSignature)
	}
	return ed25519.PublicKey(decoded), nil
}

func KeyID(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:8])
}

func validateManifest(manifest Manifest) error {
	if manifest.Format != Format || !safeReleaseValue(manifest.Version) || !validLowerHex(manifest.Commit, 40) || len(manifest.Artifacts) == 0 || len(manifest.Artifacts) > 32 {
		return fmt.Errorf("%w: header is invalid", ErrInvalidManifest)
	}
	publishedAt, err := time.Parse(time.RFC3339, manifest.PublishedAt)
	if err != nil || publishedAt.Location() != time.UTC || publishedAt.Format(time.RFC3339) != manifest.PublishedAt {
		return fmt.Errorf("%w: published_at must be canonical UTC RFC3339", ErrInvalidManifest)
	}
	names := make(map[string]struct{}, len(manifest.Artifacts))
	targets := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if filepath.Base(artifact.Name) != artifact.Name || !safeReleaseValue(artifact.Name) || !validPlatform(artifact.OS, artifact.Arch) || artifact.Size <= 0 || artifact.Size > 16<<30 || !validLowerHex(artifact.SHA256, 64) {
			return fmt.Errorf("%w: artifact is invalid", ErrInvalidManifest)
		}
		target := artifact.OS + "/" + artifact.Arch
		if _, exists := names[artifact.Name]; exists {
			return fmt.Errorf("%w: duplicate artifact name", ErrInvalidManifest)
		}
		if _, exists := targets[target]; exists {
			return fmt.Errorf("%w: duplicate artifact target", ErrInvalidManifest)
		}
		names[artifact.Name] = struct{}{}
		targets[target] = struct{}{}
	}
	return nil
}

func safeReleaseValue(value string) bool {
	if value == "" || len(value) > 255 {
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

func validPlatform(targetOS, targetArch string) bool {
	return (targetOS == "linux" || targetOS == "darwin" || targetOS == "windows") && (targetArch == "amd64" || targetArch == "arm64")
}

func validLowerHex(value string, length int) bool {
	if len(value) != length || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
