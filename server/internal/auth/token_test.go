// Copyright 2026 gitsss Authors. All rights reserved.

package auth

import (
	"errors"
	"testing"
	"time"
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
