// Copyright 2026 gitsss Authors. All rights reserved.

package gittransport

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/repository"
)

const commandOutputLimit = 16 << 20

type Handler struct {
	verifier     auth.Verifier
	repositories *repository.Service
	audit        *audit.Log
	now          func() time.Time
}

func NewHandler(verifier auth.Verifier, repositories *repository.Service, log *audit.Log) *Handler {
	return &Handler{verifier: verifier, repositories: repositories, audit: log, now: time.Now}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	repositoryID, operation, ok := parsePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	service, scope, ok := requestedService(r, operation)
	if !ok {
		http.Error(w, "unsupported Git operation", http.StatusBadRequest)
		return
	}
	principal, ok := h.authenticate(w, r, scope)
	if !ok {
		return
	}
	resolved, err := h.repositories.Resolve(repositoryID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	requestID := newRequestID()
	if err := h.appendAudit(principal.TokenID, "git."+service+".authorize", repositoryID, requestID, "authorized"); err != nil {
		http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
		return
	}
	protocol := r.Header.Get("Git-Protocol")
	if len(protocol) > 1024 || strings.ContainsAny(protocol, "\r\n\x00") {
		http.Error(w, "invalid Git protocol header", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodGet {
		h.advertise(w, r, principal.TokenID, requestID, service, resolved, protocol)
		return
	}
	h.rpc(w, r, principal.TokenID, requestID, service, resolved, protocol)
}

func (h *Handler) advertise(w http.ResponseWriter, r *http.Request, actor, requestID, service string, target repository.Repository, protocol string) {
	command := gitCommand(r, protocol, service, "--stateless-rpc", "--advertise-refs", target.Path)
	output, err := boundedOutput(command)
	if err != nil {
		_ = h.appendAudit(actor, "git."+service, target.ID, requestID, "failed")
		http.Error(w, "Git reference advertisement failed", http.StatusInternalServerError)
		return
	}
	if err := h.appendAudit(actor, "git."+service, target.ID, requestID, "advertised"); err != nil {
		http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-git-"+service+"-advertisement")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	header := "# service=git-" + service + "\n"
	_, _ = fmt.Fprintf(w, "%04x%s0000", len(header)+4, header)
	_, _ = w.Write(output)
}

func (h *Handler) rpc(w http.ResponseWriter, r *http.Request, actor, requestID, service string, target repository.Repository, protocol string) {
	command := gitCommand(r, protocol, service, "--stateless-rpc", target.Path)
	command.Stdin = r.Body
	if service == "receive-pack" {
		output, err := boundedOutput(command)
		result := "accepted"
		if err != nil {
			result = "failed"
		}
		if auditErr := h.appendAudit(actor, "git."+service, target.ID, requestID, result); auditErr != nil {
			http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
			return
		}
		if err != nil {
			http.Error(w, "Git receive failed", http.StatusBadRequest)
			return
		}
		writeRPCResponse(w, service, output)
		return
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		http.Error(w, "Git upload failed", http.StatusInternalServerError)
		return
	}
	var stderr bytes.Buffer
	command.Stderr = &limitedWriter{writer: &stderr, remaining: 64 << 10}
	if err := command.Start(); err != nil {
		http.Error(w, "Git upload failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-git-"+service+"-result")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, stdout)
	waitErr := command.Wait()
	result := "completed"
	if copyErr != nil || waitErr != nil {
		result = "failed"
	}
	_ = h.appendAudit(actor, "git."+service, target.ID, requestID, result)
}

func requestedService(r *http.Request, operation string) (string, string, bool) {
	if r.Method == http.MethodGet && operation == "info/refs" {
		switch r.URL.Query().Get("service") {
		case "git-upload-pack":
			return "upload-pack", "repositories:clone", true
		case "git-receive-pack":
			return "receive-pack", "repositories:push", true
		}
	}
	if r.Method == http.MethodPost {
		service := strings.TrimPrefix(operation, "git-")
		if service != "upload-pack" && service != "receive-pack" {
			return "", "", false
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-git-"+service+"-request" {
			return "", "", false
		}
		if service == "upload-pack" {
			return service, "repositories:clone", true
		}
		return service, "repositories:push", true
	}
	return "", "", false
}

func parsePath(requestPath string) (string, string, bool) {
	relative := strings.TrimPrefix(requestPath, "/git/")
	separator := strings.IndexByte(relative, '/')
	if relative == requestPath || separator < 1 {
		return "", "", false
	}
	repositoryID := strings.TrimSuffix(relative[:separator], ".git")
	if repositoryID == relative[:separator] || repositoryID == "" {
		return "", "", false
	}
	operation := relative[separator+1:]
	if strings.Contains(operation, "//") || operation == "" {
		return "", "", false
	}
	return repositoryID, operation, true
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request, scope string) (auth.Principal, bool) {
	token, ok := authorizationToken(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="gitsss"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return auth.Principal{}, false
	}
	principal, err := h.verifier.Verify(token, scope, h.now().UTC())
	if err == nil {
		return principal, true
	}
	if errors.Is(err, auth.ErrMissingScope) {
		http.Error(w, "repository permission denied", http.StatusForbidden)
		return auth.Principal{}, false
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="gitsss"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
	return auth.Principal{}, false
}

func authorizationToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") && len(header) > len("Bearer ") {
		token := strings.TrimPrefix(header, "Bearer ")
		return token, strings.TrimSpace(token) == token && !strings.ContainsAny(token, " \t\r\n\x00")
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return "", false
	}
	token := password
	if token == "" {
		token = username
	}
	return token, strings.TrimSpace(token) == token && token != "" && !strings.ContainsAny(token, "\r\n\x00")
}

func gitCommand(r *http.Request, protocol, service string, arguments ...string) *exec.Cmd {
	command := exec.CommandContext(r.Context(), "git", append([]string{service}, arguments...)...)
	if protocol != "" {
		command.Env = append(os.Environ(), "GIT_PROTOCOL="+protocol)
	}
	return command
}

func boundedOutput(command *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &limitedWriter{writer: &output, remaining: commandOutputLimit}
	command.Stderr = &limitedWriter{writer: &stderr, remaining: 64 << 10}
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

func writeRPCResponse(w http.ResponseWriter, service string, output []byte) {
	w.Header().Set("Content-Type", "application/x-git-"+service+"-result")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

func (h *Handler) appendAudit(actor, action, target, requestID, result string) error {
	return h.audit.Append(audit.Entry{
		Actor: actor, Action: action, Target: "repository:" + target,
		RequestID: requestID, Result: result, At: h.now().UTC(),
	})
}

type limitedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedWriter) Write(content []byte) (int, error) {
	if int64(len(content)) > w.remaining {
		return 0, errors.New("Git command output exceeded limit")
	}
	written, err := w.writer.Write(content)
	w.remaining -= int64(written)
	return written, err
}

func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("git-%d", time.Now().UnixNano())
	}
	return "git-" + hex.EncodeToString(value)
}
