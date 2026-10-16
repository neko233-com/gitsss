// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package graceful

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdditionalWebListenerServesSharedHandler(t *testing.T) {
	setWebHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}), false)
	address := "127.0.0.1:0"
	require.NoError(t, AddWebListener("tcp", address))
	t.Cleanup(func() { require.NoError(t, RemoveWebListener(address)) })

	webListeners.Lock()
	listener := webListeners.servers[address].listener
	webListeners.Unlock()
	response, err := http.Get("http://" + listener.Addr().String())
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "ok", string(body))

	require.NoError(t, RemoveWebListener(address))
}
