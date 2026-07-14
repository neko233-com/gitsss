// Copyright 2026 gitsss Authors. All rights reserved.

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	gitsssupdate "github.com/neko233-com/gitsss/server/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "keygen":
		keygen(os.Args[2:])
	case "manifest":
		manifest(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: gitsss-release <keygen|manifest> [options]")
}

func keygen(args []string) {
	flags := flag.NewFlagSet("keygen", flag.ExitOnError)
	flags.Parse(args)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(struct {
		PrivateSeed string `json:"private_seed"`
		PublicKey   string `json:"public_key"`
		KeyID       string `json:"key_id"`
	}{
		PrivateSeed: gitsssupdate.EncodePrivateSeed(privateKey),
		PublicKey:   base64.RawStdEncoding.EncodeToString(publicKey),
		KeyID:       gitsssupdate.KeyID(publicKey),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func manifest(args []string) {
	flags := flag.NewFlagSet("manifest", flag.ExitOnError)
	releaseDirectory := flags.String("release-dir", "", "absolute directory containing the six release archives")
	version := flags.String("version", "", "release version without v prefix")
	commit := flags.String("commit", "", "40-character source commit")
	publishedAt := flags.String("published-at", "", "canonical UTC RFC3339 publication time")
	privateKeyEnvironment := flags.String("private-key-env", "GITSSS_RELEASE_SIGNING_KEY", "environment variable containing the base64 Ed25519 seed")
	flags.Parse(args)
	if !validReleaseEnvironmentName(*privateKeyEnvironment) {
		fmt.Fprintln(os.Stderr, "--private-key-env must be a safe environment variable name")
		os.Exit(2)
	}
	privateKey, err := gitsssupdate.DecodePrivateSeed(os.Getenv(*privateKeyEnvironment))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	updateManifest, err := buildManifest(*releaseDirectory, *version, *commit, *publishedAt)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	manifestContent, signatureContent, err := gitsssupdate.Sign(updateManifest, privateKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if err := writeSignedManifest(*releaseDirectory, manifestContent, signatureContent, gitsssupdate.EncodePublicKey(publicKey)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("signed update manifest: %s (%s)\n", *version, gitsssupdate.KeyID(publicKey))
}

func validReleaseEnvironmentName(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}
