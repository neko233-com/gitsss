// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/stretchr/testify/assert"
)

func TestForwardedHeadersTrustBoundary(t *testing.T) {
	defer test.MockVariableValue(&setting.ReverseProxyProtoHeader, "X-Original-Proto")()
	for _, tt := range []struct {
		name, peer, xff, real, proto, wantIP, wantProto string
		limit                                           int
	}{
		{"direct spoof", "192.0.2.4:100", "198.51.100.7", "198.51.100.8", "https", "192.0.2.4:100", "", 2},
		{"trusted tunnel", "127.0.0.1:100", "198.51.100.7,127.0.0.1", "unix:", "https", "198.51.100.7:0", "https", 2},
		{"untrusted middle hop", "127.0.0.1:100", "203.0.113.8, 198.51.100.7", "203.0.113.8", "https", "198.51.100.7:0", "https", 2},
		{"bounded hops", "127.0.0.1:100", "198.51.100.7, 127.0.0.1", "198.51.100.8", "https", "127.0.0.1:0", "https", 1},
		{"invalid chain", "127.0.0.1:100", "198.51.100.7, unix:", "198.51.100.8", "https", "127.0.0.1:0", "https", 2},
		{"legacy single header", "127.0.0.1:100", "", "198.51.100.7", "https", "198.51.100.7:0", "https", 1},
		{"IPv6", "[::1]:100", "2001:db8::7", "", "https", "[2001:db8::7]:0", "https", 2},
		{"disabled", "127.0.0.1:100", "198.51.100.7", "198.51.100.8", "https", "127.0.0.1:100", "", 0},
		{"invalid scheme", "127.0.0.1:100", "198.51.100.7", "", "https,http", "198.51.100.7:0", "", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://git.example.invalid/", nil)
			req.RemoteAddr = tt.peer
			req.Header.Set("X-Forwarded-For", tt.xff)
			req.Header.Set("X-Real-IP", tt.real)
			req.Header.Set("X-Original-Proto", tt.proto)
			req.Header.Set("X-Forwarded-Proto", "http")
			req.Header.Set("X-Forwarded-Protocol", "https")
			ForwardedHeadersHandler(tt.limit, []string{"127.0.0.1/32", "::1/128"})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.wantIP, r.RemoteAddr)
				assert.Equal(t, tt.wantProto, r.Header.Get("X-Forwarded-Proto"))
				assert.Empty(t, r.Header.Get("X-Forwarded-Protocol"))
			})).ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
