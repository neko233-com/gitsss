// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	std_html "html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	packages_model "github.com/neko233-com/gitsss/models/packages"
	"github.com/neko233-com/gitsss/models/unittest"
	user_model "github.com/neko233-com/gitsss/models/user"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/test"
	"github.com/neko233-com/gitsss/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageMirrors(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.PackageMirror)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	var calls atomic.Int32
	var origin string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("Cookie"))
		switch r.URL.Path {
		case "/maven/org/example/limited/1.0/limited-1.0.jar":
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/maven/org/example/lib/1.0/lib-1.0.jar":
			_, _ = io.WriteString(w, "jar-fixture")
		case "/go/github.com/!acme/lib/@v/v1.0.0.info":
			_, _ = io.WriteString(w, `{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`)
		case "/go/sumdb/sum.golang.org/supported":
			w.WriteHeader(http.StatusOK)
		case "/cargo/se/rd/serde":
			_, _ = io.WriteString(w, `{"name":"serde","vers":"1.0.0","cksum":"fixture"}`+"\n")
		case "/crates/serde/serde-1.0.0.crate":
			_, _ = io.WriteString(w, "crate-fixture")
		case "/nuget/newtonsoft.json/index.json":
			_, _ = io.WriteString(w, `{"versions":["13.0.3"]}`)
		case "/nuget/newtonsoft.json/13.0.3/newtonsoft.json.13.0.3.nupkg":
			_, _ = io.WriteString(w, "nupkg-fixture")
		case "/composer/p2/example/library.json":
			_, _ = fmt.Fprintf(w, `{"packages":{"example/library":[{"name":"example/library","version":"1.0.0","dist":{"type":"zip","url":%q,"shasum":"fixture"}}]}}`, origin+"/files/library.zip")
		case "/files/library.zip":
			_, _ = io.WriteString(w, "composer-fixture")
		case "/npm/@scope/lib":
			_, _ = fmt.Fprintf(w, `{"name":"@scope/lib","dist-tags":{"latest":"1.0.0"},"versions":{"1.0.0":{"name":"@scope/lib","version":"1.0.0","dist":{"tarball":%q,"integrity":"sha512-fixture"}}}}`, origin+"/files/lib.tgz")
		case "/pypi/simple/lib/":
			_, _ = fmt.Fprintf(w, `<html><a href="%s/files/lib.whl#sha256=fixture" data-core-metadata="true">lib.whl</a></html>`, origin)
		case "/files/lib.tgz", "/files/lib.whl":
			_, _ = io.WriteString(w, "artifact-fixture")
		case "/files/lib.whl.metadata":
			_, _ = io.WriteString(w, "Metadata-Version: 2.1\nName: lib\nVersion: 1.0\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	origin = upstream.URL
	setting.PackageMirror.Enabled = true
	setting.PackageMirror.MavenURL = origin + "/maven"
	setting.PackageMirror.GoURL = origin + "/go"
	setting.PackageMirror.NpmURL = origin + "/npm"
	setting.PackageMirror.PyPIURL = origin + "/pypi/simple"
	setting.PackageMirror.CargoIndexURL = origin + "/cargo"
	setting.PackageMirror.CargoFilesURL = origin + "/crates"
	setting.PackageMirror.NuGetURL = origin + "/nuget"
	setting.PackageMirror.ComposerURL = origin + "/composer"
	setting.PackageMirror.ComposerFileHosts = nil
	setting.PackageMirror.PrivatePatterns = []string{"private-*", "private-*/**", "github.com/private/**", "com.private:*", "@private/*"}
	setting.PackageMirror.MetadataTTL = time.Minute
	setting.PackageMirror.ArtifactTTL = time.Hour
	setting.PackageMirror.Timeout = time.Second
	setting.PackageMirror.MaxSize = 1024 * 1024
	setting.PackageMirror.MaxFileSize = 128 * 1024
	root := "/api/packages/" + user.Name

	t.Run("Maven", func(t *testing.T) {
		resource := root + "/maven/org/example/lib/1.0/lib-1.0.jar"
		for _, state := range []string{"MISS", "HIT"} {
			resp := MakeRequest(t, NewRequest(t, "GET", resource).AddBasicAuth(user.Name), http.StatusOK)
			assert.Equal(t, "jar-fixture", resp.Body.String())
			assert.Equal(t, state, resp.Header().Get("X-GitSSS-Mirror"))
		}
		resp := MakeRequest(t, NewRequest(t, "HEAD", resource), http.StatusOK)
		assert.Equal(t, "11", resp.Header().Get("Content-Length"))
		assert.Empty(t, resp.Body.String())
		resp = MakeRequest(t, NewRequest(t, "GET", root+"/maven/org/example/limited/1.0/limited-1.0.jar"), http.StatusTooManyRequests)
		assert.Equal(t, "30", resp.Header().Get("Retry-After"))
	})
	t.Run("GoAndSumDB", func(t *testing.T) {
		resource := root + "/go/github.com/!acme/lib/@v/v1.0.0.info"
		resp := MakeRequest(t, NewRequest(t, "GET", resource), http.StatusOK)
		assert.Contains(t, resp.Body.String(), `"Version":"v1.0.0"`)
		resource = root + "/go/sumdb/sum.golang.org/supported"
		MakeRequest(t, NewRequest(t, "GET", resource), http.StatusOK)
		resp = MakeRequest(t, NewRequest(t, "GET", resource), http.StatusOK)
		assert.Equal(t, "HIT", resp.Header().Get("X-GitSSS-Mirror"))
	})
	t.Run("SubPathDeployment", func(t *testing.T) {
		defer test.MockVariableValue(&setting.AppSubURL, "/gitsss")()
		resp := MakeRequest(t, NewRequest(t, "GET", root+"/maven/org/example/lib/1.0/lib-1.0.jar"), http.StatusOK)
		assert.Equal(t, "HIT", resp.Header().Get("X-GitSSS-Mirror"))
	})
	t.Run("CargoSparseIndexAndCrate", func(t *testing.T) {
		for _, resource := range []string{"/cargo/se/rd/serde", "/cargo/api/v1/crates/serde/1.0.0/download"} {
			for _, state := range []string{"MISS", "HIT"} {
				resp := MakeRequest(t, NewRequest(t, "GET", root+resource), http.StatusOK)
				assert.Equal(t, state, resp.Header().Get("X-GitSSS-Mirror"))
				assert.NotEmpty(t, resp.Body.String())
			}
		}
	})
	t.Run("NuGetFlatContainer", func(t *testing.T) {
		for _, resource := range []string{"/nuget/package/newtonsoft.json/index.json", "/nuget/package/newtonsoft.json/13.0.3/newtonsoft.json.13.0.3.nupkg"} {
			for _, state := range []string{"MISS", "HIT"} {
				resp := MakeRequest(t, NewRequest(t, "GET", root+resource), http.StatusOK)
				assert.Equal(t, state, resp.Header().Get("X-GitSSS-Mirror"))
				assert.NotEmpty(t, resp.Body.String())
			}
		}
	})
	t.Run("ComposerMetadataAndArchive", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", root+"/composer/p2/example/library.json"), http.StatusOK)
		var metadata map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &metadata))
		versions := metadata["packages"].(map[string]any)["example/library"].([]any)
		dist := versions[0].(map[string]any)["dist"].(map[string]any)
		assert.Equal(t, "fixture", dist["shasum"])
		link, err := url.Parse(dist["url"].(string))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(link.Path, root+"/composer/mirror/"))
		for _, state := range []string{"MISS", "HIT"} {
			resp = MakeRequest(t, NewRequest(t, "GET", link.RequestURI()), http.StatusOK)
			assert.Equal(t, "composer-fixture", resp.Body.String())
			assert.Equal(t, state, resp.Header().Get("X-GitSSS-Mirror"))
		}
		MakeRequest(t, NewRequest(t, "GET", strings.Replace(link.RequestURI(), "/user2/", "/user4/", 1)), http.StatusBadRequest)
	})
	t.Run("NpmScopedArtifact", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", root+"/npm/@scope/lib"), http.StatusOK)
		var metadata map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &metadata))
		versions := metadata["versions"].(map[string]any)
		dist := versions["1.0.0"].(map[string]any)["dist"].(map[string]any)
		assert.Equal(t, "sha512-fixture", dist["integrity"])
		link, err := url.Parse(dist["tarball"].(string))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(link.Path, root+"/npm/-/mirror/"))
		resp = MakeRequest(t, NewRequest(t, "GET", link.RequestURI()).SetHeader("Range", "bytes=0-7"), http.StatusPartialContent)
		assert.Equal(t, "artifact", resp.Body.String())
		MakeRequest(t, NewRequest(t, "GET", strings.Replace(link.RequestURI(), "/user2/", "/user4/", 1)), http.StatusBadRequest)
	})
	t.Run("PyPIArtifactAndMetadata", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", root+"/pypi/simple/lib"), http.StatusOK)
		match := regexp.MustCompile(`href="([^"]+)"`).FindStringSubmatch(resp.Body.String())
		require.Len(t, match, 2)
		link, err := url.Parse(std_html.UnescapeString(match[1]))
		require.NoError(t, err)
		assert.Equal(t, "sha256=fixture", link.Fragment)
		resp = MakeRequest(t, NewRequest(t, "GET", link.RequestURI()), http.StatusOK)
		assert.Equal(t, "artifact-fixture", resp.Body.String())
		resp = MakeRequest(t, NewRequest(t, "GET", link.RequestURI()+".metadata"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "Metadata-Version: 2.1")
	})
	t.Run("PrivateNamesAndLocalPrecedence", func(t *testing.T) {
		before := calls.Load()
		for _, relative := range []string{"/maven/com/private/lib/1.0/lib-1.0.jar", "/go/github.com/private/lib/@v/v1.0.0.info", "/go/sumdb/sum.golang.org/lookup/github.com/private/lib@v1.0.0", "/npm/@private/lib", "/pypi/simple/private-lib", "/cargo/pr/iv/private-lib", "/nuget/package/private-lib/index.json", "/composer/p2/private-company/lib.json"} {
			MakeRequest(t, NewRequest(t, "GET", root+relative), http.StatusNotFound)
		}
		content := test.WriteZipArchive(map[string]string{"github.com/Acme/lib@v1.1.0/go.mod": "module github.com/Acme/lib\n"}).Bytes()
		MakeRequest(t, NewRequestWithBody(t, "PUT", root+"/go/upload", bytes.NewReader(content)).AddBasicAuth(user.Name), http.StatusCreated)
		MakeRequest(t, NewRequest(t, "GET", root+"/go/github.com/!acme/lib/@v/v1.0.0.info"), http.StatusNotFound)
		assert.Equal(t, before, calls.Load())
		pvs, err := packages_model.GetVersionsByPackageName(t.Context(), user.ID, packages_model.TypeGo, "github.com/Acme/lib")
		require.NoError(t, err)
		assert.Len(t, pvs, 1)
	})
	t.Run("PrivateOwnerRequiresAuthorization", func(t *testing.T) {
		before := calls.Load()
		for _, relative := range []string{"/maven/org/example/lib/1.0/lib-1.0.jar", "/go/github.com/!acme/lib/@v/v1.0.0.info", "/npm/@scope/lib", "/pypi/simple/lib", "/cargo/se/rd/serde", "/nuget/package/newtonsoft.json/index.json", "/composer/p2/example/library.json"} {
			MakeRequest(t, NewRequest(t, "GET", "/api/packages/private_org35"+relative), http.StatusUnauthorized)
		}
		assert.Equal(t, before, calls.Load())
	})
	t.Run("DisabledDoesNotReadCacheOrFetch", func(t *testing.T) {
		setting.PackageMirror.Enabled = false
		before := calls.Load()
		MakeRequest(t, NewRequest(t, "GET", root+"/maven/org/example/lib/1.0/lib-1.0.jar"), http.StatusNotFound)
		assert.Equal(t, before, calls.Load())
	})
}
