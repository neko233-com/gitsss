// Copyright 2026 gitsss Authors. All rights reserved.

package auth

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestVerifierVerify(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	verifier := NewVerifier(NewRecord("agent-1", "secret", now.Add(time.Hour), "dashboard:read"))

	principal, err := verifier.Verify("secret", "dashboard:read", now)
	if err != nil {
		t.Fatalf("Verify returned an error: %v", err)
	}
	if principal.TokenID != "agent-1" {
		t.Fatalf("TokenID = %q, want %q", principal.TokenID, "agent-1")
	}
}

func TestPersistentVerifierIssuesRevokesAndSurvivesRestart(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	verifier, err := NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := verifier.EnsureBootstrap(strings.Repeat("b", minimumTokenBytes), now, now.Add(24*time.Hour), "auth:read", "auth:write")
	if err != nil || !seeded {
		t.Fatalf("bootstrap = %v, %v", seeded, err)
	}
	issued, err := verifier.Issue("desktop-admin", []string{"dashboard:read", "auth:read"}, now, now.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, "gitsss_") || strings.Contains(string(mustTokenDocuments(t, store)), issued.Token) {
		t.Fatal("issued token was malformed or persisted as plaintext")
	}
	reopened, err := NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	if principal, err := reopened.Verify(issued.Token, "dashboard:read", now); err != nil || principal.TokenID != "desktop-admin" {
		t.Fatalf("reopened verification = %#v, %v", principal, err)
	}
	if err := reopened.Revoke("desktop-admin", "bootstrap", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Verify(issued.Token, "dashboard:read", now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked verification error = %v", err)
	}
	if err := reopened.Revoke("bootstrap", "bootstrap", now); !errors.Is(err, ErrSelfRevoke) {
		t.Fatalf("self revoke error = %v", err)
	}
}

func TestPersistentVerifierRequiresBootstrapOnlyWhenEmpty(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	verifier, err := NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.EnsureBootstrap("short", now, now.Add(time.Hour), "auth:write"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("short bootstrap error = %v", err)
	}
	if seeded, err := verifier.EnsureBootstrap(strings.Repeat("s", minimumTokenBytes), now, now.Add(time.Hour), "auth:write"); err != nil || !seeded {
		t.Fatalf("bootstrap = %v, %v", seeded, err)
	}
	if seeded, err := verifier.EnsureBootstrap("", now, now.Add(time.Hour), "auth:write"); err != nil || seeded {
		t.Fatalf("existing bootstrap = %v, %v", seeded, err)
	}
}

func mustTokenDocuments(t *testing.T, store *state.Store) []byte {
	t.Helper()
	documents, err := store.List(tokenStateNamespace)
	if err != nil {
		t.Fatal(err)
	}
	var combined []byte
	for _, document := range documents {
		combined = append(combined, document.Payload...)
	}
	return combined
}

func TestVerifierRejectsExpiredAndUnscopedTokens(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	expired := NewVerifier(NewRecord("expired", "secret", now, "dashboard:read"))
	if _, err := expired.Verify("secret", "dashboard:read", now); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("Verify error = %v, want ErrExpiredToken", err)
	}

	limited := NewVerifier(NewRecord("limited", "secret", time.Time{}, "dashboard:read"))
	if _, err := limited.Verify("secret", "events:write", now); !errors.Is(err, ErrMissingScope) {
		t.Fatalf("Verify error = %v, want ErrMissingScope", err)
	}
}

func TestVerifierIssuesBoundedRoleTokenAndPersistsRole(t *testing.T) {
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	verifier, err := NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := verifier.IssueForRole("artist", "contributor", now, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if issued.Role != "contributor" || len(issued.Scopes) == 0 {
		t.Fatalf("issued role token = %#v", issued)
	}
	if _, err := verifier.Verify(issued.Token, "repositories:push", now); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(issued.Token, "auth:write", now); !errors.Is(err, ErrMissingScope) {
		t.Fatalf("role escalation error = %v", err)
	}
	if _, err := verifier.IssueForRole("too-long", "contributor", now, 91*24*time.Hour); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("excessive role duration error = %v", err)
	}
	reopened, err := NewPersistentVerifier(store)
	if err != nil {
		t.Fatal(err)
	}
	tokens := reopened.Tokens()
	if len(tokens) != 1 || tokens[0].Role != "contributor" {
		t.Fatalf("persisted role tokens = %#v", tokens)
	}
}

func TestRolesReturnIndependentLeastPrivilegeTemplates(t *testing.T) {
	roles := Roles()
	if len(roles) != 6 || roles[0].ID != "administrator" {
		t.Fatalf("roles = %#v", roles)
	}
	viewer, err := Role("viewer")
	if err != nil {
		t.Fatal(err)
	}
	viewer.Scopes[0] = "auth:write"
	unchanged, err := Role("viewer")
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range unchanged.Scopes {
		if scope == "auth:write" || scope == "repositories:push" {
			t.Fatalf("viewer scopes were mutable or excessive: %#v", unchanged.Scopes)
		}
	}
}
