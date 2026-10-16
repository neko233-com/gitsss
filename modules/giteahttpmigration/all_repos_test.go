// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListRepositoryRefsPagesAllVisibleRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"name":"client","owner":{"login":"engineering"}},{"name":"config","owner":{"login":"engineering"}}]}`))
	}))
	t.Cleanup(server.Close)
	client := newAPIClient(server.URL, "token", server.Client())
	refs, err := client.listRepositoryRefs(context.Background())
	require.NoError(t, err)
	require.Equal(t, []RepositoryRef{
		{Owner: "engineering", Name: "client"},
		{Owner: "engineering", Name: "config"},
	}, refs)
}

func TestNewAllowsEmptyRepoListWhenImportingEverything(t *testing.T) {
	migrator, err := New(Options{
		SourceURL: "http://source.test", SourceToken: "source",
		TargetURL: "http://target.test", TargetToken: "target",
		AllRepositories: true,
	})
	require.NoError(t, err)
	require.True(t, migrator.allRepos)
	require.Empty(t, migrator.repos)
}
