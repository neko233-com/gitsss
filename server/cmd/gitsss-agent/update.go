// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

type updateVerificationResult struct {
	Version     string                `json:"version"`
	Commit      string                `json:"commit"`
	PublishedAt string                `json:"published_at"`
	KeyID       string                `json:"key_id"`
	Artifact    gitsssupdate.Artifact `json:"artifact"`
}

func updateVerify(args []string) {
	flags := flag.NewFlagSet("update-verify", flag.ExitOnError)
	manifestPath := flags.String("manifest", "", "absolute path to canonical update-manifest.json")
	signaturePath := flags.String("signature", "", "absolute path to update-manifest.sig")
	publicKeyPath := flags.String("public-key", "", "absolute path to the trusted Ed25519 public key")
	packagePath := flags.String("package", "", "absolute path to the downloaded update package")
	targetOS := flags.String("os", runtime.GOOS, "expected target operating system")
	targetArch := flags.String("arch", runtime.GOARCH, "expected target architecture")
	flags.Parse(args)
	result, err := verifyUpdateFiles(*manifestPath, *signaturePath, *publicKeyPath, *packagePath, *targetOS, *targetArch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func verifyUpdateFiles(manifestPath, signaturePath, publicKeyPath, packagePath, targetOS, targetArch string) (updateVerificationResult, error) {
	for _, filePath := range []string{manifestPath, signaturePath, publicKeyPath, packagePath} {
		if strings.TrimSpace(filePath) == "" || !filepath.IsAbs(filePath) {
			return updateVerificationResult{}, errors.New("manifest, signature, public key and package must use absolute paths")
		}
	}
	manifestContent, err := readBoundedUpdateFile(manifestPath, 1<<20)
	if err != nil {
		return updateVerificationResult{}, err
	}
	signatureContent, err := readBoundedUpdateFile(signaturePath, 4<<10)
	if err != nil {
		return updateVerificationResult{}, err
	}
	publicKeyContent, err := readBoundedUpdateFile(publicKeyPath, 1<<10)
	if err != nil {
		return updateVerificationResult{}, err
	}
	manifest, signature, err := gitsssupdate.Verify(manifestContent, signatureContent, publicKeyContent)
	if err != nil {
		return updateVerificationResult{}, err
	}
	artifact, err := gitsssupdate.VerifyArtifact(manifest, packagePath, targetOS, targetArch)
	if err != nil {
		return updateVerificationResult{}, err
	}
	return updateVerificationResult{
		Version: manifest.Version, Commit: manifest.Commit, PublishedAt: manifest.PublishedAt,
		KeyID: signature.KeyID, Artifact: artifact,
	}, nil
}

func readBoundedUpdateFile(filePath string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximum {
		return nil, fmt.Errorf("update metadata must be a regular file up to %d bytes", maximum)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	currentInfo, err := os.Lstat(filePath)
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		return nil, errors.New("update metadata changed while opening")
	}
	content, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maximum {
		return nil, fs.ErrInvalid
	}
	return content, nil
}
