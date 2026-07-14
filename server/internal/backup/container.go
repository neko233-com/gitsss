// Copyright 2026 gitsss Authors. All rights reserved.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const (
	containerMagic  = "GITSSSB1"
	headerSize      = 40
	saltSize        = 16
	noncePrefixSize = 8
	chunkSize       = 1 << 20
	keyIterations   = 600_000
	manifestName    = "manifest.json"
	maxManifestSize = 32 << 20
)

var ErrIntegrity = errors.New("backup integrity verification failed")

func Export(outputPath, passphrase string, plan Plan) error {
	if passphrase == "" {
		return fmt.Errorf("%w: passphrase is required", ErrInvalidBackup)
	}
	if err := validateManifest(plan.Manifest); err != nil {
		return err
	}
	if len(plan.sources) != len(plan.Manifest.Entries) {
		return fmt.Errorf("%w: source and manifest counts differ", ErrInvalidBackup)
	}
	absolute, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absolute); err == nil {
		return fmt.Errorf("%w: output already exists", ErrInvalidBackup)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	directory := filepath.Dir(absolute)
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: output directory does not exist", ErrInvalidBackup)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(absolute)+".partial-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	completed := false
	defer func() {
		if !completed {
			temporary.Close()
			os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if err := writeContainer(temporary, passphrase, plan); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, absolute); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	completed = true
	return nil
}

func writeContainer(destination io.Writer, passphrase string, plan Plan) error {
	encrypted, err := newEncryptWriter(destination, passphrase)
	if err != nil {
		return err
	}
	compressed := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(compressed)
	manifest, err := json.Marshal(plan.Manifest)
	if err != nil {
		return err
	}
	if len(manifest) > maxManifestSize {
		return fmt.Errorf("%w: manifest exceeds %d bytes", ErrInvalidBackup, maxManifestSize)
	}
	if err := archive.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(manifest)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := archive.Write(manifest); err != nil {
		return err
	}
	for index := range plan.sources {
		if err := writeSource(archive, plan.sources[index]); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	return encrypted.Close()
}

func writeSource(archive *tar.Writer, source sourceEntry) error {
	linkInfo, err := os.Lstat(source.path)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	file, err := os.Open(source.path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != source.entry.Size {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	header := &tar.Header{
		Name: source.entry.Path, Mode: 0o600, Size: source.entry.Size,
		ModTime: info.ModTime(), Typeflag: tar.TypeReg,
	}
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	hasher := sha256.New()
	written, err := io.CopyN(io.MultiWriter(archive, hasher), file, source.entry.Size)
	if err != nil || written != source.entry.Size {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	var extra [1]byte
	if count, err := file.Read(extra[:]); count != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	after, err := file.Stat()
	if err != nil || after.Size() != source.entry.Size || fmt.Sprintf("%x", hasher.Sum(nil)) != source.entry.SHA256 {
		return fmt.Errorf("%w: %s", ErrSourceChanged, source.entry.Path)
	}
	return nil
}

type encryptWriter struct {
	destination io.Writer
	aead        cipher.AEAD
	header      []byte
	noncePrefix [noncePrefixSize]byte
	buffer      []byte
	counter     uint32
	closed      bool
}

func newEncryptWriter(destination io.Writer, passphrase string) (*encryptWriter, error) {
	header := make([]byte, headerSize)
	copy(header, containerMagic)
	if _, err := rand.Read(header[12 : 12+saltSize]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(header[28 : 28+noncePrefixSize]); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint32(header[36:40], keyIterations)
	header[8] = 1
	// Version occupies byte 8; chunk size is stored in bytes 9..11 as a 24-bit value.
	header[9] = byte((chunkSize >> 16) & 0xff)
	header[10] = byte((chunkSize >> 8) & 0xff)
	header[11] = byte(chunkSize & 0xff)
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
	if _, err := destination.Write(header); err != nil {
		return nil, err
	}
	writer := &encryptWriter{destination: destination, aead: aead, header: header, buffer: make([]byte, 0, chunkSize)}
	copy(writer.noncePrefix[:], header[28:36])
	return writer, nil
}

func (writer *encryptWriter) Write(content []byte) (int, error) {
	if writer.closed {
		return 0, errors.New("write to closed backup container")
	}
	written := 0
	for len(content) > 0 {
		available := chunkSize - len(writer.buffer)
		if available > len(content) {
			available = len(content)
		}
		writer.buffer = append(writer.buffer, content[:available]...)
		content = content[available:]
		written += available
		if len(writer.buffer) == chunkSize {
			if err := writer.writeFrame(0, writer.buffer); err != nil {
				return written, err
			}
			writer.buffer = writer.buffer[:0]
		}
	}
	return written, nil
}

func (writer *encryptWriter) Close() error {
	if writer.closed {
		return nil
	}
	writer.closed = true
	if len(writer.buffer) > 0 {
		if err := writer.writeFrame(0, writer.buffer); err != nil {
			return err
		}
	}
	return writer.writeFrame(1, nil)
}

func (writer *encryptWriter) writeFrame(flag byte, plaintext []byte) error {
	if writer.counter == math.MaxUint32 {
		return fmt.Errorf("%w: container is too large", ErrInvalidBackup)
	}
	var frameHeader [5]byte
	frameHeader[0] = flag
	binary.BigEndian.PutUint32(frameHeader[1:], uint32(len(plaintext)))
	nonce := make([]byte, writer.aead.NonceSize())
	copy(nonce, writer.noncePrefix[:])
	binary.BigEndian.PutUint32(nonce[8:], writer.counter)
	aad := make([]byte, 0, len(writer.header)+len(frameHeader))
	aad = append(aad, writer.header...)
	aad = append(aad, frameHeader[:]...)
	ciphertext := writer.aead.Seal(nil, nonce, plaintext, aad)
	if _, err := writer.destination.Write(frameHeader[:]); err != nil {
		return err
	}
	if _, err := writer.destination.Write(ciphertext); err != nil {
		return err
	}
	writer.counter++
	return nil
}
