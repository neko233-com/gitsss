// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package docs bundles the offline product manual.
package docs

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed site operations architecture
var manualFiles embed.FS

// Handler serves the bundled manual with English as its default entry point.
func Handler(prefix, routePrefix string) http.Handler {
	prefix = strings.TrimSuffix(prefix, "/")
	routePrefix = strings.TrimSuffix(routePrefix, "/")
	files := http.StripPrefix(routePrefix, http.FileServer(http.FS(manualFiles)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == routePrefix || r.URL.Path == routePrefix+"/" {
			http.Redirect(w, r, prefix+"/site/en/", http.StatusTemporaryRedirect)
			return
		}
		// The web router normalizes trailing slashes; FileServer needs the original directory URL.
		originalPath, _, _ := strings.Cut(r.RequestURI, "?")
		if strings.HasSuffix(originalPath, "/") && !strings.HasSuffix(r.URL.Path, "/") {
			r = r.Clone(r.Context())
			r.URL.Path += "/"
			r.URL.RawPath = ""
		}
		files.ServeHTTP(w, r)
	})
}
