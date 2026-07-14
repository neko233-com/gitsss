// Copyright 2026 gitsss Authors. All rights reserved.

package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVaultEncryptsAndSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "gitsss.key")
	vault, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	secret := json.RawMessage(`{"password":"super-secret"}`)
	first, err := vault.Seal("notifications", "email", secret)
	if err != nil {
		t.Fatal(err)
	}
	second, err := vault.Seal("notifications", "email", secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) || strings.Contains(string(first), "super-secret") {
		t.Fatalf("encrypted documents exposed or reused ciphertext: %s", first)
	}
	reopened, err := OpenVault(path)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := reopened.Open("notifications", "email", first)
	if err != nil || string(plaintext) != string(secret) {
		t.Fatalf("Open = %s, %v", plaintext, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != vaultKeyBytes || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		t.Fatalf("master key mode/size = %v/%d", info.Mode().Perm(), info.Size())
	}
}

func TestVaultRejectsWrongIdentityAndTampering(t *testing.T) {
	vault, err := OpenVault(filepath.Join(t.TempDir(), "gitsss.key"))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := vault.Seal("notifications", "email", json.RawMessage(`{"password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Open("notifications", "wecom", envelope); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("wrong identity error = %v", err)
	}
	envelope[len(envelope)-3] ^= 1
	if _, err := vault.Open("notifications", "email", envelope); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("tamper error = %v", err)
	}
}
