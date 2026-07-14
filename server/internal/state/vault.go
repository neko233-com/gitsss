// Copyright 2026 gitsss Authors. All rights reserved.

package state

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	vaultKeyBytes      = 32
	vaultFormatVersion = 1
	maxSecretBytes     = 512 << 10
)

var ErrInvalidCiphertext = errors.New("invalid encrypted state document")

type encryptedDocument struct {
	Version    int    `json:"v"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type Vault struct {
	key [vaultKeyBytes]byte
}

func OpenVault(path string) (*Vault, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("state master key path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, err
	}
	key, err := readOrCreateVaultKey(absolute)
	if err != nil {
		return nil, err
	}
	vault := &Vault{}
	copy(vault.key[:], key)
	clear(key)
	return vault, nil
}

func (vault *Vault) Seal(namespace, id string, plaintext json.RawMessage) (json.RawMessage, error) {
	if vault == nil || !validKey(namespace, 64) || !validKey(id, 256) || len(plaintext) == 0 || len(plaintext) > maxSecretBytes || !json.Valid(plaintext) {
		return nil, ErrInvalidDocument
	}
	aead, err := vault.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, vaultAAD(namespace, id))
	envelope, err := json.Marshal(encryptedDocument{
		Version: vaultFormatVersion, Nonce: base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext),
	})
	if err != nil {
		return nil, err
	}
	return envelope, nil
}

func (vault *Vault) Open(namespace, id string, envelope json.RawMessage) (json.RawMessage, error) {
	if vault == nil || !validKey(namespace, 64) || !validKey(id, 256) || len(envelope) == 0 || len(envelope) > maxDocumentBytes || !json.Valid(envelope) {
		return nil, ErrInvalidCiphertext
	}
	var document encryptedDocument
	if err := json.Unmarshal(envelope, &document); err != nil || document.Version != vaultFormatVersion {
		return nil, ErrInvalidCiphertext
	}
	aead, err := vault.aead()
	if err != nil {
		return nil, err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(document.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return nil, ErrInvalidCiphertext
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(document.Ciphertext)
	if err != nil || len(ciphertext) < aead.Overhead() {
		return nil, ErrInvalidCiphertext
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, vaultAAD(namespace, id))
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxSecretBytes || !json.Valid(plaintext) {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}

func (vault *Vault) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(vault.key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func readOrCreateVaultKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() != vaultKeyBytes {
			return nil, errors.New("state master key must be a 32-byte regular file")
		}
		if err := protectFile(path); err != nil {
			return nil, err
		}
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(key) != vaultKeyBytes {
			return nil, errors.New("state master key changed while reading")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, vaultKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return readOrCreateVaultKey(path)
	}
	if err != nil {
		return nil, err
	}
	remove := true
	defer func() {
		file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := protectFile(path); err != nil {
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if err := protectFile(path); err != nil {
		return nil, err
	}
	remove = false
	return key, nil
}

func vaultAAD(namespace, id string) []byte {
	return []byte(fmt.Sprintf("gitsss-state-v1\x00%s\x00%s", namespace, id))
}
