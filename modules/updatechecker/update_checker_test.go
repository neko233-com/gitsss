package updatechecker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitSSSUpdateCheckerDisabledWithoutEndpoint(t *testing.T) {
	require.NoError(t, GitSSSUpdateChecker(context.Background(), ""))
}

func TestGitSSSUpdateCheckerRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	err := GitSSSUpdateChecker(context.Background(), server.URL)
	assert.ErrorContains(t, err, "HTTP 502")
}
