// Copyright 2026 gitsss Authors. All rights reserved.

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

const (
	tokenStateNamespace = "auth-tokens"
	minimumTokenBytes   = 24
	maximumScopes       = 64
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("expired token")
	ErrMissingScope  = errors.New("missing token scope")
	ErrTokenExists   = errors.New("token ID already exists")
	ErrTokenNotFound = errors.New("token not found")
	ErrSelfRevoke    = errors.New("a token cannot revoke itself")
	ErrState         = errors.New("authentication state unavailable")
)

type Record struct {
	ID        string
	Hash      [sha256.Size]byte
	ExpiresAt time.Time
	Scopes    map[string]struct{}
	CreatedAt time.Time
	RevokedAt time.Time
}

type TokenInfo struct {
	ID        string    `json:"id"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	RevokedAt time.Time `json:"revoked_at,omitempty"`
}

type IssuedToken struct {
	TokenInfo
	Token string `json:"token"`
}

type persistedRecord struct {
	ID        string    `json:"id"`
	Hash      string    `json:"hash"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	RevokedAt time.Time `json:"revoked_at,omitempty"`
}

type Principal struct {
	TokenID string
}

type verifierState struct {
	mu      sync.RWMutex
	records []Record
	store   *state.Store
}

type Verifier struct {
	state *verifierState
}

func NewRecord(id, token string, expiresAt time.Time, scopes ...string) Record {
	allowedScopes := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		allowedScopes[scope] = struct{}{}
	}
	return Record{ID: id, Hash: sha256.Sum256([]byte(token)), ExpiresAt: expiresAt, Scopes: allowedScopes}
}

func NewVerifier(records ...Record) Verifier {
	return Verifier{state: &verifierState{records: append([]Record(nil), records...)}}
}

func NewPersistentVerifier(store *state.Store) (Verifier, error) {
	if store == nil {
		return Verifier{}, errors.New("authentication state store is required")
	}
	documents, err := store.List(tokenStateNamespace)
	if err != nil {
		return Verifier{}, fmt.Errorf("%w: %v", ErrState, err)
	}
	records := make([]Record, 0, len(documents))
	for _, document := range documents {
		var persisted persistedRecord
		if err := json.Unmarshal(document.Payload, &persisted); err != nil || document.ID != persisted.ID {
			return Verifier{}, fmt.Errorf("%w: corrupt token %q", ErrState, document.ID)
		}
		record, err := decodeRecord(persisted)
		if err != nil {
			return Verifier{}, fmt.Errorf("%w: corrupt token %q", ErrState, document.ID)
		}
		records = append(records, record)
	}
	sort.Slice(records, func(left, right int) bool { return records[left].ID < records[right].ID })
	return Verifier{state: &verifierState{records: records, store: store}}, nil
}

func (v Verifier) EnsureBootstrap(token string, now, expiresAt time.Time, scopes ...string) (bool, error) {
	if v.state == nil {
		return false, ErrState
	}
	v.state.mu.Lock()
	defer v.state.mu.Unlock()
	if len(v.state.records) != 0 {
		return false, nil
	}
	if len(token) < minimumTokenBytes || strings.TrimSpace(token) != token || strings.ContainsAny(token, "\r\n\x00") || !expiresAt.After(now) {
		return false, ErrInvalidToken
	}
	record := NewRecord("bootstrap", token, expiresAt.UTC(), scopes...)
	record.CreatedAt = now.UTC()
	if !validRecord(record, true) {
		return false, ErrInvalidToken
	}
	if err := v.state.persist(record); err != nil {
		return false, err
	}
	v.state.records = append(v.state.records, record)
	return true, nil
}

func (v Verifier) Issue(id string, scopes []string, now, expiresAt time.Time) (IssuedToken, error) {
	if v.state == nil || !validID(id) || !expiresAt.After(now) || expiresAt.After(now.Add(365*24*time.Hour)) {
		return IssuedToken{}, ErrInvalidToken
	}
	allowedScopes, err := normalizeScopes(scopes)
	if err != nil {
		return IssuedToken{}, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return IssuedToken{}, err
	}
	token := "gitsss_" + base64.RawURLEncoding.EncodeToString(secret)
	clear(secret)
	record := Record{
		ID: id, Hash: sha256.Sum256([]byte(token)), ExpiresAt: expiresAt.UTC(), Scopes: allowedScopes,
		CreatedAt: now.UTC(),
	}
	v.state.mu.Lock()
	defer v.state.mu.Unlock()
	for _, existing := range v.state.records {
		if existing.ID == id {
			return IssuedToken{}, ErrTokenExists
		}
	}
	if err := v.state.persist(record); err != nil {
		return IssuedToken{}, err
	}
	v.state.records = append(v.state.records, record)
	return IssuedToken{TokenInfo: tokenInfo(record), Token: token}, nil
}

func (v Verifier) Revoke(id, actorID string, now time.Time) error {
	if v.state == nil || !validID(id) {
		return ErrInvalidToken
	}
	if id == actorID {
		return ErrSelfRevoke
	}
	v.state.mu.Lock()
	defer v.state.mu.Unlock()
	for index, record := range v.state.records {
		if record.ID != id {
			continue
		}
		if !record.RevokedAt.IsZero() {
			return nil
		}
		record.RevokedAt = now.UTC()
		if err := v.state.persist(record); err != nil {
			return err
		}
		v.state.records[index] = record
		return nil
	}
	return ErrTokenNotFound
}

func (v Verifier) Tokens() []TokenInfo {
	if v.state == nil {
		return nil
	}
	v.state.mu.RLock()
	defer v.state.mu.RUnlock()
	tokens := make([]TokenInfo, 0, len(v.state.records))
	for _, record := range v.state.records {
		tokens = append(tokens, tokenInfo(record))
	}
	sort.Slice(tokens, func(left, right int) bool {
		if tokens[left].CreatedAt.Equal(tokens[right].CreatedAt) {
			return tokens[left].ID < tokens[right].ID
		}
		return tokens[left].CreatedAt.After(tokens[right].CreatedAt)
	})
	return tokens
}

func (v Verifier) Verify(token, requiredScope string, now time.Time) (Principal, error) {
	if v.state == nil || strings.TrimSpace(token) == "" {
		return Principal{}, ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(token))
	v.state.mu.RLock()
	defer v.state.mu.RUnlock()
	for _, record := range v.state.records {
		if subtle.ConstantTimeCompare(record.Hash[:], hash[:]) != 1 {
			continue
		}
		if !record.RevokedAt.IsZero() {
			return Principal{}, ErrInvalidToken
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

func (state *verifierState) persist(record Record) error {
	if state.store == nil {
		return nil
	}
	persisted := persistedRecord{
		ID: record.ID, Hash: hex.EncodeToString(record.Hash[:]), Scopes: scopeList(record.Scopes),
		CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt, RevokedAt: record.RevokedAt,
	}
	payload, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := state.store.Put(tokenStateNamespace, record.ID, payload); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func decodeRecord(persisted persistedRecord) (Record, error) {
	hash, err := hex.DecodeString(persisted.Hash)
	if err != nil || len(hash) != sha256.Size {
		return Record{}, ErrInvalidToken
	}
	scopes, err := normalizeScopes(persisted.Scopes)
	if err != nil {
		return Record{}, err
	}
	record := Record{
		ID: persisted.ID, ExpiresAt: persisted.ExpiresAt, Scopes: scopes,
		CreatedAt: persisted.CreatedAt, RevokedAt: persisted.RevokedAt,
	}
	copy(record.Hash[:], hash)
	if !validRecord(record, true) {
		return Record{}, ErrInvalidToken
	}
	return record, nil
}

func validRecord(record Record, requireCreatedAt bool) bool {
	if !validID(record.ID) || len(record.Scopes) == 0 || len(record.Scopes) > maximumScopes {
		return false
	}
	if requireCreatedAt && record.CreatedAt.IsZero() {
		return false
	}
	if !record.RevokedAt.IsZero() && record.RevokedAt.Before(record.CreatedAt) {
		return false
	}
	return record.ExpiresAt.IsZero() || record.ExpiresAt.After(record.CreatedAt)
}

func validID(id string) bool {
	trimmed := strings.TrimSpace(id)
	return id == trimmed && id != "" && len(id) <= 128 && !strings.ContainsAny(id, "\r\n\x00/")
}

func normalizeScopes(scopes []string) (map[string]struct{}, error) {
	if len(scopes) == 0 || len(scopes) > maximumScopes {
		return nil, ErrInvalidToken
	}
	normalized := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		parts := strings.Split(scope, ":")
		if len(parts) != 2 || !validScopePart(parts[0]) || !validScopePart(parts[1]) {
			return nil, ErrInvalidToken
		}
		normalized[scope] = struct{}{}
	}
	return normalized, nil
}

func validScopePart(part string) bool {
	if part == "" || len(part) > 64 {
		return false
	}
	for _, character := range part {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func tokenInfo(record Record) TokenInfo {
	return TokenInfo{
		ID: record.ID, Scopes: scopeList(record.Scopes), CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt, RevokedAt: record.RevokedAt,
	}
}

func scopeList(scopes map[string]struct{}) []string {
	values := make([]string, 0, len(scopes))
	for scope := range scopes {
		values = append(values, scope)
	}
	sort.Strings(values)
	return values
}
