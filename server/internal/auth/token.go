// Copyright 2026 gitsss Authors. All rights reserved.

package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
	ErrMissingScope = errors.New("missing token scope")
)

type Record struct {
	ID        string
	Hash      [sha256.Size]byte
	ExpiresAt time.Time
	Scopes    map[string]struct{}
}

type Principal struct {
	TokenID string
}

type Verifier struct {
	records []Record
}

func NewRecord(id, token string, expiresAt time.Time, scopes ...string) Record {
	allowedScopes := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		allowedScopes[scope] = struct{}{}
	}
	return Record{ID: id, Hash: sha256.Sum256([]byte(token)), ExpiresAt: expiresAt, Scopes: allowedScopes}
}

func NewVerifier(records ...Record) Verifier {
	return Verifier{records: append([]Record(nil), records...)}
}

func (v Verifier) Verify(token, requiredScope string, now time.Time) (Principal, error) {
	if strings.TrimSpace(token) == "" {
		return Principal{}, ErrInvalidToken
	}

	hash := sha256.Sum256([]byte(token))
	for _, record := range v.records {
		if subtle.ConstantTimeCompare(record.Hash[:], hash[:]) != 1 {
			continue
		}
		if !record.ExpiresAt.IsZero() && !record.ExpiresAt.After(now) {
			return Principal{}, ErrExpiredToken
		}
		if _, ok := record.Scopes[requiredScope]; !ok {
			return Principal{}, ErrMissingScope
		}
		return Principal{TokenID: record.ID}, nil
	}

	return Principal{}, ErrInvalidToken
}
