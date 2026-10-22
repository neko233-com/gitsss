// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package orgshare plans organization file sharing without touching Git or LFS paths.
package orgshare

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultMaxBytes is the per-organization share cap, 100 GiB.
	DefaultMaxBytes int64 = 100 << 30
	// DefaultMaxFileBytes leaves room for a language or IDE installer inside that cap.
	DefaultMaxFileBytes int64 = 32 << 30
	// DefaultExpireDays removes a new object after 30 days unless it is marked permanent.
	DefaultExpireDays = 30
	// DefaultReserveBytes stays free on the share volume so the disk is not filled to zero.
	DefaultReserveBytes int64 = 1 << 30
	DefaultParallel           = 4
	MaxParallel               = 8
	DefaultChunkBytes   int64 = 8 << 20
	MaxChunkBytes       int64 = 32 << 20
)

// QuotaInput is the size check before a new object is accepted.
type QuotaInput struct {
	Used     int64
	Incoming int64
	MaxBytes int64
	MaxFile  int64
}

// CheckQuota rejects an object that would pass the organization or file cap.
func CheckQuota(input QuotaInput) error {
	if input.Incoming < 0 || input.Used < 0 {
		return errors.New("share size must not be negative")
	}
	if input.MaxFile > 0 && input.Incoming > input.MaxFile {
		return fmt.Errorf("file is larger than the %d byte limit", input.MaxFile)
	}
	if input.MaxBytes > 0 && input.Used+input.Incoming > input.MaxBytes {
		return fmt.Errorf("organization share is over the %d byte capacity", input.MaxBytes)
	}
	return nil
}

// ContentRange is an inclusive byte range.
type ContentRange struct {
	Start int64
	End   int64
	Total int64
}

// ParseContentRange reads "bytes START-END/TOTAL". The end byte is inclusive.
func ParseContentRange(header string) (ContentRange, error) {
	header = strings.TrimSpace(header)
	value, ok := strings.CutPrefix(strings.ToLower(header), "bytes ")
	if !ok {
		return ContentRange{}, errors.New("content range must start with bytes")
	}
	span, totalText, ok := strings.Cut(value, "/")
	if !ok {
		return ContentRange{}, errors.New("content range must include the total size")
	}
	startText, endText, ok := strings.Cut(span, "-")
	if !ok {
		return ContentRange{}, errors.New("content range must include start and end")
	}
	start, err := strconv.ParseInt(startText, 10, 64)
	if err != nil {
		return ContentRange{}, fmt.Errorf("content range start: %w", err)
	}
	end, err := strconv.ParseInt(endText, 10, 64)
	if err != nil {
		return ContentRange{}, fmt.Errorf("content range end: %w", err)
	}
	total, err := strconv.ParseInt(totalText, 10, 64)
	if err != nil {
		return ContentRange{}, fmt.Errorf("content range total: %w", err)
	}
	if start < 0 || end < start || total <= end {
		return ContentRange{}, errors.New("content range is outside the file")
	}
	return ContentRange{Start: start, End: end, Total: total}, nil
}

// Length is the number of bytes in the inclusive range.
func (r ContentRange) Length() int64 {
	return r.End - r.Start + 1
}

// SafeParts splits a relative upload path and rejects escapes.
func SafeParts(raw string) ([]string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return nil, errors.New("share name is empty")
	}
	if utf8.RuneCountInString(raw) > 512 {
		return nil, errors.New("share name is too long")
	}
	parts := strings.Split(raw, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.Contains(part, "\x00") || utf8.RuneCountInString(part) > 255 {
			return nil, fmt.Errorf("share name %q is not allowed", raw)
		}
	}
	return parts, nil
}

// CleanJoin joins safe parts. The caller must pass parts from SafeParts.
func CleanJoin(parts []string) string {
	return path.Join(parts...)
}

// Expiry returns the deadline for a new object. Zero days means it does not expire.
func Expiry(now time.Time, days int) time.Time {
	if days <= 0 {
		return time.Time{}
	}
	return now.UTC().AddDate(0, 0, days)
}

// Deadline returns when a new object expires. Permanent objects and a non-positive day count do not expire.
func Deadline(now time.Time, days int, permanent bool) time.Time {
	if permanent {
		return time.Time{}
	}
	return Expiry(now, days)
}

// Expired reports whether a stored deadline has passed. A zero deadline never expires.
func Expired(deadline, now time.Time) bool {
	return !deadline.IsZero() && !deadline.After(now)
}

// CanRead is true for organization members.
func CanRead(isMember bool) bool { return isMember }

// CanWrite is true for organization members.
func CanWrite(isMember bool) bool { return isMember }

// CanDelete allows the uploader and organization owners to remove one object.
func CanDelete(isOwner, isUploader bool) bool { return isOwner || isUploader }
