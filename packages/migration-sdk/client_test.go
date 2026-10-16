// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsedPaging(t *testing.T) {
	resp := newResponse(&http.Response{
		Header: http.Header{
			"Link": []string{
				strings.Join(
					[]string{
						`<https://try.gitea.io/api/v1/repos/gitea/go-sdk/issues/1/comments?page=3>; rel="next"`,
						`<https://try.gitea.io/api/v1/repos/gitea/go-sdk/issues/1/comments?page=4>; rel="last"`,
						`<https://try.gitea.io/api/v1/repos/gitea/go-sdk/issues/1/comments?page=1>; rel="first"`,
						`<https://try.gitea.io/api/v1/repos/gitea/go-sdk/issues/1/comments?page=1>; rel="prev"`,
					}, ",",
				),
			},
		},
	})

	assert.Equal(t, 1, resp.FirstPage)
	assert.Equal(t, 1, resp.PrevPage)
	assert.Equal(t, 3, resp.NextPage)
	assert.Equal(t, 4, resp.LastPage)
}

func TestClientMethodContextUsesRequestContextWithoutMutatingClient(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, SetGiteaVersion("1.26.0"))
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, _, err = client.GetMyUserInfo(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, requests.Load())

	_, _, err = client.GetMyUserInfo(t.Context())
	assert.False(t, errors.Is(err, context.Canceled))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, requests.Load())
}

func TestClientMethodContextKeepsConfiguredServerVersion(t *testing.T) {
	t.Parallel()

	var versionRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			versionRequests.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, SetGiteaVersion("1.26.0"))
	assert.NoError(t, err)

	err = client.CheckServerVersionConstraint(t.Context(), ">= 1.11.0")
	assert.NoError(t, err)
	assert.Zero(t, versionRequests.Load())
}

func TestNewClientDoesNotFetchServerVersion(t *testing.T) {
	t.Parallel()

	var versionRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			versionRequests.Add(1)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"1.26.0"}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL)
	assert.NoError(t, err)
	assert.Zero(t, versionRequests.Load())
}
