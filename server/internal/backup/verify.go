// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

type Verification struct {
	Manifest        Manifest `json:"manifest"`
	Files           int      `json:"files"`
	Bytes           int64    `json:"bytes"`
	ContainerBytes  int64    `json:"container_bytes"`
	ContainerSHA256 string   `json:"container_sha256"`
}

func Verify(inputPath, passphrase string) (Verification, error) {
	return processContainer(inputPath, passphrase, nil)
}

type entryConsumer func(Entry, io.Reader) error

func processContainer(inputPath, passphrase string, consume entryConsumer) (Verification, error) {
	if passphrase == "" {
		return Verification{}, fmt.Errorf("%w: passphrase is required", ErrInvalidBackup)
	}
	file, err := os.Open(inputPath)
	if err != nil {
		return Verification{}, err
	}
	defer file.Close()
	hasher := sha256.New()
	var containerCounter byteCounter
	container := io.TeeReader(file, io.MultiWriter(hasher, &containerCounter))
	decrypted, err := newDecryptReader(container, passphrase)
	if err != nil {
		return Verification{}, integrityError(err)
	}
	compressed, err := gzip.NewReader(decrypted)
	if err != nil {
		return Verification{}, integrityError(err)
	}
	archive := tar.NewReader(compressed)
	header, err := archive.Next()
	if err != nil || header.Name != manifestName || header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxManifestSize {
		return Verification{}, integrityError(err)
	}
	manifestBytes, err := io.ReadAll(io.LimitReader(archive, maxManifestSize+1))
	if err != nil || int64(len(manifestBytes)) != header.Size {
		return Verification{}, integrityError(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Verification{}, integrityError(err)
	}
	if err := validateManifest(manifest); err != nil {
		return Verification{}, integrityError(err)
	}
	verification := Verification{Manifest: manifest}
	for _, expected := range manifest.Entries {
		header, err := archive.Next()
		if err != nil || header.Name != expected.Path || header.Typeflag != tar.TypeReg || header.Size != expected.Size {
			return Verification{}, integrityError(err)
		}
		hasher := sha256.New()
		var counter byteCounter
		content := io.TeeReader(archive, io.MultiWriter(hasher, &counter))
		if consume != nil {
			if err := consume(expected, content); err != nil {
				return Verification{}, err
			}
		}
		if _, err := io.Copy(io.Discard, content); err != nil || int64(counter) != expected.Size || hex.EncodeToString(hasher.Sum(nil)) != expected.SHA256 {
			return Verification{}, integrityError(err)
		}
		verification.Files++
		verification.Bytes += int64(counter)
	}
	if _, err := archive.Next(); !errors.Is(err, io.EOF) {
		return Verification{}, integrityError(err)
	}
	if _, err := io.Copy(io.Discard, compressed); err != nil {
		return Verification{}, integrityError(err)
	}
	if err := compressed.Close(); err != nil {
		return Verification{}, integrityError(err)
	}
	if err := decrypted.Finish(); err != nil {
		return Verification{}, integrityError(err)
	}
	verification.ContainerBytes = int64(containerCounter)
	verification.ContainerSHA256 = hex.EncodeToString(hasher.Sum(nil))
	return verification, nil
}

type byteCounter int64

func (counter *byteCounter) Write(content []byte) (int, error) {
	*counter += byteCounter(len(content))
	return len(content), nil
}

func validateManifest(manifest Manifest) error {
	if manifest.Version != 1 || manifest.CreatedAt.IsZero() || strings.TrimSpace(manifest.Source.AssessmentID) == "" {
		return fmt.Errorf("%w: unsupported or incomplete manifest", ErrInvalidBackup)
	}
	if !validConfigDigest(manifest.Source.ConfigDigest) || len(manifest.Entries) == 0 || len(manifest.Entries) > maxManifestEntries {
		return fmt.Errorf("%w: invalid manifest bounds", ErrInvalidBackup)
	}
	previous := ""
	for _, entry := range manifest.Entries {
		if !safeArchivePath(entry.Path) || !safeKind(entry.Kind) || entry.Size < 0 || !validDigest(entry.SHA256) {
			return fmt.Errorf("%w: invalid manifest entry", ErrInvalidBackup)
		}
		if previous != "" && entry.Path <= previous {
			return fmt.Errorf("%w: manifest paths must be unique and sorted", ErrInvalidBackup)
		}
		previous = entry.Path
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validConfigDigest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validDigest(strings.TrimPrefix(value, "sha256:"))
}

func integrityError(cause error) error {
	if cause == nil {
		return ErrIntegrity
	}
	return fmt.Errorf("%w: %v", ErrIntegrity, cause)
}

type decryptReader struct {
	source      io.Reader
	aead        cipher.AEAD
	header      []byte
	noncePrefix [noncePrefixSize]byte
	buffer      []byte
	offset      int
	counter     uint32
	final       bool
}

func newDecryptReader(source io.Reader, passphrase string) (*decryptReader, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(source, header); err != nil {
		return nil, err
	}
	if string(header[:8]) != containerMagic || header[8] != 1 || decodeChunkSize(header[9:12]) != chunkSize || binary.BigEndian.Uint32(header[36:40]) != keyIterations {
		return nil, ErrIntegrity
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, header[12:28], keyIterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	reader := &decryptReader{source: source, aead: aead, header: header}
	copy(reader.noncePrefix[:], header[28:36])
	return reader, nil
}

func (reader *decryptReader) Read(destination []byte) (int, error) {
	for {
		if reader.offset < len(reader.buffer) {
			count := copy(destination, reader.buffer[reader.offset:])
			reader.offset += count
			return count, nil
		}
		if reader.final {
			return 0, io.EOF
		}
		if err := reader.readFrame(); err != nil {
			return 0, err
		}
	}
}

func (reader *decryptReader) Finish() error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	if !reader.final {
		return ErrIntegrity
	}
	return nil
}

func (reader *decryptReader) readFrame() error {
	if reader.counter == math.MaxUint32 {
		return ErrIntegrity
	}
	var frameHeader [5]byte
	if _, err := io.ReadFull(reader.source, frameHeader[:]); err != nil {
		return ErrIntegrity
	}
	flag := frameHeader[0]
	length := binary.BigEndian.Uint32(frameHeader[1:])
	if (flag != 0 && flag != 1) || length > chunkSize || (flag == 0 && length == 0) || (flag == 1 && length != 0) {
		return ErrIntegrity
	}
	ciphertext := make([]byte, int(length)+reader.aead.Overhead())
	if _, err := io.ReadFull(reader.source, ciphertext); err != nil {
		return ErrIntegrity
	}
	nonce := make([]byte, reader.aead.NonceSize())
	copy(nonce, reader.noncePrefix[:])
	binary.BigEndian.PutUint32(nonce[8:], reader.counter)
	aad := make([]byte, 0, len(reader.header)+len(frameHeader))
	aad = append(aad, reader.header...)
	aad = append(aad, frameHeader[:]...)
	plaintext, err := reader.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return ErrIntegrity
	}
	reader.counter++
	if flag == 1 {
		var trailing [1]byte
		count, err := reader.source.Read(trailing[:])
		if count != 0 || !errors.Is(err, io.EOF) {
			return ErrIntegrity
		}
		reader.final = true
		reader.buffer = nil
		reader.offset = 0
		return nil
	}
	reader.buffer = plaintext
	reader.offset = 0
	return nil
}

func decodeChunkSize(value []byte) uint32 {
	return uint32(value[0])<<16 | uint32(value[1])<<8 | uint32(value[2])
}
