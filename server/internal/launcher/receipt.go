// Copyright 2026 gitsss Authors. All rights reserved.

package launcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const ReceiptFormat = "gitsss-launcher-start-v1"

var (
	ErrInvalidReceipt = errors.New("invalid launcher start receipt")
	receiptName       = regexp.MustCompile(`^started-([0-9]{20})-([0-9]{20})-([0-9]{10})\.json$`)
)

type StartReceipt struct {
	Format     string `json:"format"`
	Generation uint64 `json:"generation"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
}

func PublishStarted(directory string, generation uint64, pid int, now time.Time) (string, error) {
	receipt := StartReceipt{
		Format: ReceiptFormat, Generation: generation, PID: pid,
		StartedAt: now.UTC().Format(time.RFC3339Nano),
	}
	content, err := canonicalReceipt(receipt)
	if err != nil {
		return "", err
	}
	if !cleanAbsolute(directory) {
		return "", fmt.Errorf("%w: receipt directory must be a clean absolute path", ErrInvalidReceipt)
	}
	if err := validateStateDirectory(directory); err != nil {
		return "", err
	}
	filePath := filepath.Join(directory, receiptFilename(receipt))
	if err := atomicStateFile(filePath, content); err != nil {
		return "", err
	}
	return filePath, nil
}

func StartedAfter(directory string, generation uint64, after time.Time) (bool, error) {
	if !cleanAbsolute(directory) || generation == 0 || after.IsZero() {
		return false, fmt.Errorf("%w: directory, generation and threshold are required", ErrInvalidReceipt)
	}
	if err := validateStateDirectory(directory); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		matches := receiptName.FindStringSubmatch(entry.Name())
		if entry.IsDir() || len(matches) != 4 {
			continue
		}
		fileGeneration, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil || fileGeneration != generation {
			continue
		}
		content, err := readStateFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return false, err
		}
		var receipt StartReceipt
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&receipt); err != nil {
			return false, fmt.Errorf("%w: %v", ErrInvalidReceipt, err)
		}
		canonical, err := canonicalReceipt(receipt)
		if err != nil || !bytes.Equal(content, canonical) || receiptFilename(receipt) != entry.Name() {
			return false, fmt.Errorf("%w: receipt is not canonical or filename mismatches", ErrInvalidReceipt)
		}
		startedAt, _ := time.Parse(time.RFC3339Nano, receipt.StartedAt)
		if !startedAt.Before(after.UTC()) {
			return true, nil
		}
	}
	return false, nil
}

func canonicalReceipt(receipt StartReceipt) ([]byte, error) {
	startedAt, err := time.Parse(time.RFC3339Nano, receipt.StartedAt)
	if receipt.Format != ReceiptFormat || receipt.Generation == 0 || receipt.PID <= 0 || err != nil || startedAt.Location() != time.UTC || startedAt.Format(time.RFC3339Nano) != receipt.StartedAt {
		return nil, fmt.Errorf("%w: receipt fields are invalid", ErrInvalidReceipt)
	}
	content, err := json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func receiptFilename(receipt StartReceipt) string {
	startedAt, _ := time.Parse(time.RFC3339Nano, receipt.StartedAt)
	return fmt.Sprintf("started-%020d-%020d-%010d.json", receipt.Generation, startedAt.UnixNano(), receipt.PID)
}
