// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/setting"
)

const (
	loginWindow        = time.Minute
	loginFailureWindow = 15 * time.Minute
	loginProofWindow   = 15 * time.Minute
	loginMaxEntries    = 32768
	loginMaxProofs     = 4096
	loginMaxConcurrent = 16
)

// ErrLoginChallenge requests a human check without locking an account or a shared IP.
var ErrLoginChallenge = errors.New("login verification required")

type LoginRateLimitError struct{ RetryAfter int }

func (e *LoginRateLimitError) Error() string {
	return "too many authentication attempts; retry later or use an access token"
}

func LoginErrorStatus(err error) int {
	if _, ok := errors.AsType[*LoginRateLimitError](err); ok {
		return http.StatusTooManyRequests
	}
	return http.StatusUnauthorized
}

type (
	loginRequestKey     struct{}
	interactiveLoginKey struct{}
	interactiveLogin    struct{ challengePassed bool }
)

// WithLoginRequest uses the address already validated by the trusted-proxy middleware.
func WithLoginRequest(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, loginRequestKey{}, req)
}

func WithInteractiveLogin(ctx context.Context, challengePassed bool) context.Context {
	return context.WithValue(ctx, interactiveLoginKey{}, interactiveLogin{challengePassed})
}

type loginBucket struct {
	window   time.Time
	expires  time.Time
	attempts int
	failures int
}

type loginGuard struct {
	mu       sync.Mutex
	buckets  map[[32]byte]*loginBucket
	proofs   map[[32]byte]time.Time
	secret   string
	now      func() time.Time
	nextGC   time.Time
	inflight int
}

func newLoginGuard() *loginGuard {
	return &loginGuard{buckets: make(map[[32]byte]*loginBucket), proofs: make(map[[32]byte]time.Time), secret: rand.Text(), now: time.Now}
}

var passwordLoginGuard = newLoginGuard()

func normalizeRemoteAddr(remote string) string {
	remote = strings.TrimSpace(remote)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if addr, err := netip.ParseAddr(strings.Trim(remote, "[]")); err == nil {
		return addr.Unmap().String()
	}
	return remote
}

func loginKey(parts ...string) [32]byte {
	h := sha256.New()
	for _, part := range parts {
		_ = binary.Write(h, binary.BigEndian, uint64(len(part)))
		_, _ = h.Write([]byte(part))
	}
	return [32]byte(h.Sum(nil))
}

func (g *loginGuard) keys(username, remote string) [3][32]byte {
	username = strings.ToLower(strings.TrimSpace(username))
	remote = normalizeRemoteAddr(remote)
	return [3][32]byte{loginKey("account", username), loginKey("pair", username, remote), loginKey("source", remote)}
}

func (g *loginGuard) proof(username, password string) [32]byte {
	h := hmac.New(sha256.New, []byte(g.secret))
	key := loginKey(strings.ToLower(strings.TrimSpace(username)), password)
	_, _ = h.Write(key[:])
	return [32]byte(h.Sum(nil))
}

func (g *loginGuard) collect(now time.Time) {
	if now.Before(g.nextGC) {
		return
	}
	for k, v := range g.buckets {
		if !now.Before(v.expires) {
			delete(g.buckets, k)
		}
	}
	for k, v := range g.proofs {
		if !now.Before(v) {
			delete(g.proofs, k)
		}
	}
	g.nextGC = now.Add(time.Minute)
}

func (g *loginGuard) challengeLocked(keys [3][32]byte, now time.Time) bool {
	for i, key := range keys {
		b := g.buckets[key]
		if b == nil || !now.Before(b.expires) {
			continue
		}
		if b.failures >= [3]int{5, 5, 30}[i] {
			return true
		}
		if now.Before(b.window.Add(loginWindow)) && b.attempts >= [3]int{10, 5, 60}[i] {
			return true
		}
	}
	return len(g.buckets)+3 > loginMaxEntries
}

func (g *loginGuard) challenge(username, password, remote string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.collect(now)
	if password != "" && now.Before(g.proofs[g.proof(username, password)]) {
		return false
	}
	return g.challengeLocked(g.keys(username, remote), now)
}

func LoginChallengeRequired(ctx context.Context, username, password string) bool {
	if !setting.LoginProtectionEnabled {
		return false
	}
	req, _ := ctx.Value(loginRequestKey{}).(*http.Request)
	return req != nil && passwordLoginGuard.challenge(username, password, req.RemoteAddr)
}

// begin reserves attempts before password hashing, including concurrent requests.
// A proof only bypasses throttling: the password and second factor are still verified.
func (g *loginGuard) begin(username, password, remote string, interactive, verified bool) (func(bool), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.collect(now)
	keys, proof := g.keys(username, remote), g.proof(username, password)
	known := now.Before(g.proofs[proof])
	if !known {
		if interactive {
			if !verified && g.challengeLocked(keys, now) {
				return nil, ErrLoginChallenge
			}
		} else {
			for i, key := range keys {
				b := g.buckets[key]
				if b != nil && now.Before(b.expires) && now.Before(b.window.Add(loginWindow)) && b.attempts >= [3]int{30, 10, 120}[i] {
					return nil, &LoginRateLimitError{RetryAfter: max(1, int(b.window.Add(loginWindow).Sub(now).Seconds())+1)}
				}
			}
			if len(g.buckets)+3 > loginMaxEntries {
				return nil, &LoginRateLimitError{RetryAfter: 60}
			}
		}
	}
	if g.inflight >= loginMaxConcurrent {
		return nil, &LoginRateLimitError{RetryAfter: 1}
	}
	g.inflight++
	for _, key := range keys {
		if known {
			break
		}
		b := g.buckets[key]
		if b == nil || !now.Before(b.expires) {
			if len(g.buckets) >= loginMaxEntries {
				continue
			}
			b = &loginBucket{window: now, expires: now.Add(loginFailureWindow)}
			g.buckets[key] = b
		}
		if !now.Before(b.window.Add(loginWindow)) {
			b.window, b.attempts = now, 0
		}
		b.attempts = min(b.attempts+1, 1000000)
	}
	var once sync.Once
	return func(success bool) {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.inflight--
			if success {
				if !known {
					delete(g.buckets, keys[0])
					delete(g.buckets, keys[1])
				}
				if len(g.proofs) < loginMaxProofs {
					g.proofs[proof] = g.now().Add(loginProofWindow)
				}
			} else {
				delete(g.proofs, proof)
				for _, key := range keys {
					if b := g.buckets[key]; b != nil {
						b.failures = min(b.failures+1, 1000000)
					}
				}
			}
		})
	}, nil
}

func beginPasswordLogin(ctx context.Context, username, password string) (func(bool), error) {
	req, _ := ctx.Value(loginRequestKey{}).(*http.Request)
	if req == nil || !setting.LoginProtectionEnabled {
		return func(bool) {}, nil
	}
	interactive, ok := ctx.Value(interactiveLoginKey{}).(interactiveLogin)
	return passwordLoginGuard.begin(username, password, req.RemoteAddr, ok, interactive.challengePassed)
}

// AllowSecondFactor limits guesses only after a valid password established the account.
func AllowSecondFactor(userID int64) error {
	if !setting.LoginProtectionEnabled {
		return nil
	}
	g := passwordLoginGuard
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.collect(now)
	key := loginKey("second-factor", string(binary.BigEndian.AppendUint64(nil, uint64(userID))))
	b := g.buckets[key]
	if b == nil || !now.Before(b.expires) {
		if len(g.buckets) >= loginMaxEntries {
			return &LoginRateLimitError{RetryAfter: 60}
		}
		b = &loginBucket{window: now, expires: now.Add(loginWindow)}
		g.buckets[key] = b
	}
	if b.attempts >= 10 {
		return &LoginRateLimitError{RetryAfter: max(1, int(b.expires.Sub(now).Seconds())+1)}
	}
	b.attempts++
	return nil
}
