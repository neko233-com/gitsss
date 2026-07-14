// Copyright 2026 gitsss Authors. All rights reserved.

package takeover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const freezeProbeTimeout = time.Second

type FreezeEvidence struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Network   string    `json:"network"`
	Address   string    `json:"address"`
	Offline   bool      `json:"offline"`
	Finding   string    `json:"finding,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func CheckFreeze(options Options) (FreezeEvidence, error) {
	dialer := &net.Dialer{Timeout: freezeProbeTimeout}
	return checkFreezeWithDial(options, dialer.DialContext)
}

func checkFreezeWithDial(options Options, dial func(context.Context, string, string) (net.Conn, error)) (FreezeEvidence, error) {
	if strings.TrimSpace(options.AgentID) == "" || strings.TrimSpace(options.ConfigPath) == "" {
		return FreezeEvidence{}, fmt.Errorf("%w: agent ID and config path are required", ErrInvalidConfig)
	}
	configPath, configuration, err := readConfiguration(options.ConfigPath)
	if err != nil {
		return FreezeEvidence{}, err
	}
	workPath := resolveWorkPath(configPath, options.WorkPath)
	network, address, err := freezeEndpoint(configuration, workPath)
	if err != nil {
		return FreezeEvidence{}, err
	}
	digest, _ := configurationDigest(configuration)
	idHash := sha256.Sum256([]byte(strings.TrimSpace(options.AgentID) + "\x00" + digest + "\x00" + network + "\x00" + address))
	evidence := FreezeEvidence{
		ID: "freeze-" + hex.EncodeToString(idHash[:8]), AgentID: strings.TrimSpace(options.AgentID),
		Network: network, Address: address, CheckedAt: time.Now().UTC(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), freezeProbeTimeout)
	defer cancel()
	connection, dialErr := dial(ctx, network, address)
	if dialErr == nil {
		connection.Close()
		evidence.Finding = "旧实例的本地监听仍可连接，请先停止旧实例"
		return evidence, nil
	}
	if freezeDialProvesOffline(network, dialErr) {
		evidence.Offline = true
		return evidence, nil
	}
	evidence.Finding = "无法确定旧实例是否已停止；超时、权限错误或网络不可达不能作为停服证据"
	return evidence, nil
}

func freezeEndpoint(configuration ini, workPath string) (string, string, error) {
	protocol := strings.ToLower(strings.TrimSpace(value(configuration, "server", "PROTOCOL")))
	if protocol == "" {
		protocol = "http"
	}
	if protocol == "http+unix" || protocol == "fcgi+unix" {
		socket := strings.TrimSpace(value(configuration, "server", "UNIX_SOCKET"))
		if socket == "" {
			return "", "", fmt.Errorf("%w: UNIX_SOCKET is required for %s", ErrInvalidConfig, protocol)
		}
		if !filepath.IsAbs(socket) {
			socket = filepath.Join(workPath, socket)
		}
		return "unix", filepath.Clean(socket), nil
	}
	if protocol != "http" && protocol != "https" && protocol != "fcgi" {
		return "", "", fmt.Errorf("%w: unsupported source server protocol %q", ErrInvalidConfig, protocol)
	}
	host := strings.TrimSpace(value(configuration, "server", "HTTP_ADDR"))
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" || host == "[::]" {
		host = "::1"
	}
	port := strings.TrimSpace(value(configuration, "server", "HTTP_PORT"))
	if port == "" {
		port = "3000"
	}
	if parsed, err := net.LookupPort("tcp", port); err != nil || parsed < 1 || parsed > 65535 {
		return "", "", fmt.Errorf("%w: invalid source HTTP_PORT", ErrInvalidConfig)
	}
	return "tcp", net.JoinHostPort(strings.Trim(host, "[]"), port), nil
}

func freezeDialProvesOffline(network string, err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var number syscall.Errno
	if errors.As(err, &number) && number == syscall.Errno(10061) {
		return true
	}
	return network == "unix" && errors.Is(err, os.ErrNotExist)
}
