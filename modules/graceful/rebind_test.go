// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRebindListenerKeepsAcceptingOnTheNewPort(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	listener := &rebindListener{current: first}
	accepted := make(chan string, 2)
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 2 {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 8)
			n, _ := io.ReadFull(conn, buf[:4])
			accepted <- string(buf[:n])
			_ = conn.Close()
		}
	})

	dial := func(addr, payload string) {
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		_, err = io.WriteString(conn, payload)
		require.NoError(t, err)
		_ = conn.Close()
	}
	dial(first.Addr().String(), "old1")
	require.Eventually(t, func() bool {
		return len(accepted) == 1
	}, time.Second, 10*time.Millisecond)

	second, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	secondAddr := second.Addr().String()
	require.NoError(t, second.Close())
	require.NoError(t, listener.Rebind("tcp", secondAddr))
	dial(secondAddr, "new2")
	require.Eventually(t, func() bool {
		return len(accepted) == 2
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, listener.Close())
	wg.Wait()
	require.Equal(t, "old1", <-accepted)
	require.Equal(t, "new2", <-accepted)
}
