// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCheckFreezeRequiresExplicitConnectionRefusal(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "app.ini")
	if err := os.WriteFile(configPath, []byte("[server]\nPROTOCOL = https\nHTTP_ADDR = 0.0.0.0\nHTTP_PORT = 3443\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := Options{AgentID: "old-server", ConfigPath: configPath}
	evidence, err := checkFreezeWithDial(options, func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "127.0.0.1:3443" {
			t.Fatalf("freeze endpoint = %s %s", network, address)
		}
		return nil, syscall.ECONNREFUSED
	})
	if err != nil || !evidence.Offline || evidence.ID == "" {
		t.Fatalf("offline evidence = %#v, %v", evidence, err)
	}
	second, err := checkFreezeWithDial(options, func(context.Context, string, string) (net.Conn, error) {
		return nil, syscall.ECONNREFUSED
	})
	if err != nil || second.ID != evidence.ID {
		t.Fatalf("freeze ID changed: %#v, %v", second, err)
	}
}

func TestCheckFreezeRejectsOnlineAndAmbiguousNetwork(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "app.ini")
	if err := os.WriteFile(configPath, []byte("[server]\nHTTP_PORT = 3000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := Options{AgentID: "old-server", ConfigPath: configPath}
	online, peer := net.Pipe()
	defer peer.Close()
	evidence, err := checkFreezeWithDial(options, func(context.Context, string, string) (net.Conn, error) {
		return online, nil
	})
	if err != nil || evidence.Offline || evidence.Finding == "" {
		t.Fatalf("online evidence = %#v, %v", evidence, err)
	}
	evidence, err = checkFreezeWithDial(options, func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("network is unreachable")
	})
	if err != nil || evidence.Offline || evidence.Finding == "" {
		t.Fatalf("ambiguous evidence = %#v, %v", evidence, err)
	}
}

func TestFreezeEndpointSupportsUnixSocket(t *testing.T) {
	configuration, err := parseINI(strings.NewReader("[server]\nPROTOCOL = http+unix\nUNIX_SOCKET = run/gitea.sock\n"))
	if err != nil {
		t.Fatal(err)
	}
	network, address, err := freezeEndpoint(configuration, "/srv/gitea")
	if err != nil || network != "unix" || address != filepath.Join("/srv/gitea", "run/gitea.sock") {
		t.Fatalf("Unix endpoint = %q %q, %v", network, address, err)
	}
}
