// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"net"
	"sync"
)

// rebindListener serves Accept from the current socket and can swap that
// socket without stopping the HTTP serve loop.
type rebindListener struct {
	mu      sync.RWMutex
	current net.Listener
	closed  bool
}

func (l *rebindListener) Accept() (net.Conn, error) {
	for {
		l.mu.RLock()
		current := l.current
		closed := l.closed
		l.mu.RUnlock()
		if closed || current == nil {
			return nil, net.ErrClosed
		}
		conn, err := current.Accept()
		if err == nil {
			return conn, nil
		}
		l.mu.RLock()
		swapped := !l.closed && l.current != current
		l.mu.RUnlock()
		if swapped {
			continue
		}
		return nil, err
	}
}

func (l *rebindListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.current == nil {
		return nil
	}
	return l.current.Close()
}

func (l *rebindListener) Addr() net.Addr {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.current == nil {
		return nil
	}
	return l.current.Addr()
}

// Rebind listens on the new address before closing the old one.
// A bind failure leaves the current listener in place.
func (l *rebindListener) Rebind(network, address string) error {
	next, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = next.Close()
		return net.ErrClosed
	}
	old := l.current
	l.current = next
	l.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

var (
	webRebindMu sync.Mutex
	webRebind   *rebindListener
)

func setWebRebind(listener *rebindListener) {
	webRebindMu.Lock()
	webRebind = listener
	webRebindMu.Unlock()
}

// RebindWeb moves the plain HTTP listener onto a new TCP address.
// Existing requests on the old port finish. New requests use the new port.
func RebindWeb(network, address string) error {
	webRebindMu.Lock()
	listener := webRebind
	webRebindMu.Unlock()
	if listener == nil {
		return net.ErrClosed
	}
	return listener.Rebind(network, address)
}
