// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package listenport plans a live HTTP port change without moving Git data.
package listenport

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ValidatePort accepts a TCP port that can be bound by the server.
func ValidatePort(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid HTTP port %q", raw)
	}
	return strconv.Itoa(port), nil
}

// RewriteRootURL moves the public site URL onto the new listen port.
// The path, scheme, and credentials are kept. An IPv6 host stays bracketed.
func RewriteRootURL(rawURL, newPort string) (string, error) {
	port, err := ValidatePort(newPort)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid root URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Hostname() == "" {
		return "", fmt.Errorf("root URL %q must be an HTTP(S) URL with a host", rawURL)
	}
	parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.String(), nil
}
