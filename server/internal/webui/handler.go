// Copyright 2026 gitsss Authors. All rights reserved.

package webui

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type handler struct {
	api  http.Handler
	root fs.FS
}

func DiscoverRoot(explicit string) (string, error) {
	candidates := make([]string, 0, 3)
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, explicit)
	} else {
		if executable, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(executable), "web"))
		}
		if workingDirectory, err := os.Getwd(); err == nil {
			candidates = append(candidates,
				filepath.Join(workingDirectory, "web", "dist"),
				filepath.Join(workingDirectory, "..", "web", "dist"),
			)
		}
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if err := validateRoot(absolute); err == nil {
			return filepath.Clean(absolute), nil
		}
	}
	return "", errors.New("React web bundle not found; build web/dist or set GITSSS_WEB_ROOT")
}

func New(api http.Handler, root string) (http.Handler, error) {
	if api == nil {
		return nil, errors.New("API handler is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := validateRoot(absolute); err != nil {
		return nil, err
	}
	return &handler{api: api, root: os.DirFS(absolute)}, nil
}

func (h *handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/api" || strings.HasPrefix(request.URL.Path, "/api/") ||
		(request.Method != http.MethodGet && request.Method != http.MethodHead) {
		h.api.ServeHTTP(response, request)
		return
	}
	relative := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
	if relative == "." || relative == "" {
		relative = "index.html"
	}
	if !fs.ValidPath(relative) {
		http.NotFound(response, request)
		return
	}
	content, err := fs.ReadFile(h.root, relative)
	if err == nil {
		h.serve(response, request, relative, content)
		return
	}
	if strings.HasPrefix(relative, "assets/") || path.Ext(relative) != "" {
		http.NotFound(response, request)
		return
	}
	content, err = fs.ReadFile(h.root, "index.html")
	if err != nil {
		http.Error(response, "web bundle unavailable", http.StatusServiceUnavailable)
		return
	}
	h.serve(response, request, "index.html", content)
}

func (h *handler) serve(response http.ResponseWriter, request *http.Request, name string, content []byte) {
	response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	response.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "DENY")
	if strings.HasPrefix(name, "assets/") {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		response.Header().Set("Cache-Control", "no-cache")
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		response.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(response, request, name, time.Time{}, bytes.NewReader(content))
}

func validateRoot(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("web root is not a plain directory: %s", root)
	}
	indexInfo, err := os.Lstat(filepath.Join(root, "index.html"))
	if err != nil {
		return err
	}
	if indexInfo.Mode()&os.ModeSymlink != 0 || !indexInfo.Mode().IsRegular() || indexInfo.Size() > 2<<20 {
		return fmt.Errorf("web index is not a small plain file: %s", root)
	}
	return nil
}
