// Copyright 2026 gitsss Authors. All rights reserved.

package lfs

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/auth"
	"github.com/neko233-com/gitsss/server/internal/repository"
	"github.com/neko233-com/gitsss/server/internal/state"
	"github.com/neko233-com/gitsss/server/internal/storage"
)

const (
	mediaType           = "application/vnd.git-lfs+json"
	maximumBatchObjects = 1000
	maximumObjectBytes  = int64(1 << 40)
)

type Options struct {
	Root      string
	PublicURL string
}

type Handler struct {
	root         string
	publicURL    string
	verifier     auth.Verifier
	repositories *repository.Service
	storage      *storage.Service
	audit        *audit.Log
	now          func() time.Time
	locks        [64]sync.Mutex
}

type batchRequest struct {
	Operation string            `json:"operation"`
	Transfers []string          `json:"transfers"`
	Objects   []objectRequest   `json:"objects"`
	HashAlgo  string            `json:"hash_algo"`
	Ref       map[string]string `json:"ref"`
}

type objectRequest struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

type action struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

type objectResponse struct {
	OID           string            `json:"oid"`
	Size          int64             `json:"size"`
	Authenticated bool              `json:"authenticated"`
	Actions       map[string]action `json:"actions,omitempty"`
	Error         *objectError      `json:"error,omitempty"`
}

type objectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewHandler(options Options, verifier auth.Verifier, repositories *repository.Service, storageService *storage.Service, log *audit.Log) (*Handler, error) {
	if strings.TrimSpace(options.Root) == "" || repositories == nil || storageService == nil || log == nil {
		return nil, errors.New("LFS dependencies are required")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(root)
	if err := secureDirectory(root); err != nil {
		return nil, err
	}
	if err := secureDirectory(filepath.Join(root, ".partial")); err != nil {
		return nil, err
	}
	publicURL := strings.TrimRight(strings.TrimSpace(options.PublicURL), "/")
	if publicURL != "" {
		parsed, err := url.Parse(publicURL)
		if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, errors.New("LFS public URL must be an HTTP(S) base URL")
		}
		publicURL = strings.TrimRight(parsed.String(), "/")
	}
	return &Handler{
		root: root, publicURL: publicURL, verifier: verifier, repositories: repositories,
		storage: storageService, audit: log, now: time.Now,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	repositoryID, resource, ok := parsePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := h.repositories.Resolve(repositoryID); err != nil {
		http.NotFound(w, r)
		return
	}
	if resource == "objects/batch" && r.Method == http.MethodPost {
		h.batch(w, r, repositoryID)
		return
	}
	if strings.HasPrefix(resource, "objects/") {
		oid := strings.TrimPrefix(resource, "objects/")
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			h.download(w, r, repositoryID, oid)
		case http.MethodPut:
			h.upload(w, r, repositoryID, oid)
		default:
			w.Header().Set("Allow", "GET, HEAD, PUT")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) batch(w http.ResponseWriter, r *http.Request, repositoryID string) {
	var request batchRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || len(request.Objects) == 0 || len(request.Objects) > maximumBatchObjects || request.HashAlgo != "" && request.HashAlgo != "sha256" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid LFS batch request"})
		return
	}
	scope := "repositories:clone"
	if request.Operation == "upload" {
		scope = "repositories:push"
	} else if request.Operation != "download" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "unsupported LFS operation"})
		return
	}
	principal, ok := h.authenticate(w, r, scope)
	if !ok {
		return
	}
	requestID := newRequestID()
	if err := h.appendAudit(principal.TokenID, "lfs.batch."+request.Operation, repositoryID, requestID, "authorized"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "audit state could not be updated"})
		return
	}
	objects := make([]objectResponse, 0, len(request.Objects))
	for _, object := range request.Objects {
		response := objectResponse{OID: object.OID, Size: object.Size, Authenticated: true}
		if !validOID(object.OID) || object.Size < 0 || object.Size > maximumObjectBytes {
			response.Error = &objectError{Code: http.StatusUnprocessableEntity, Message: "invalid LFS object"}
			objects = append(objects, response)
			continue
		}
		path := h.objectPath(object.OID)
		info, err := regularFileInfo(path)
		if request.Operation == "download" {
			if err != nil || info.Size() != object.Size {
				response.Error = &objectError{Code: http.StatusNotFound, Message: "LFS object not found"}
			} else {
				response.Actions = map[string]action{"download": h.objectAction(r, repositoryID, object.OID, false)}
			}
			objects = append(objects, response)
			continue
		}
		if err == nil && info.Size() == object.Size {
			objects = append(objects, response)
			continue
		}
		admission := h.storage.Admit(object.Size)
		if !admission.Allowed {
			response.Error = &objectError{Code: http.StatusInsufficientStorage, Message: admissionReason(admission)}
		} else {
			response.Actions = map[string]action{"upload": h.objectAction(r, repositoryID, object.OID, true)}
		}
		objects = append(objects, response)
	}
	writeLFSJSON(w, http.StatusOK, map[string]any{"transfer": "basic", "hash_algo": "sha256", "objects": objects})
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request, repositoryID, oid string) {
	if !validOID(oid) {
		http.NotFound(w, r)
		return
	}
	principal, ok := h.authenticate(w, r, "repositories:clone")
	if !ok {
		return
	}
	file, err := os.Open(h.objectPath(oid))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	if err := h.appendAudit(principal.TokenID, "lfs.download", repositoryID+":"+oid, newRequestID(), "authorized"); err != nil {
		http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"sha256:`+oid+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, oid, info.ModTime(), file)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request, repositoryID, oid string) {
	if !validOID(oid) {
		http.NotFound(w, r)
		return
	}
	principal, ok := h.authenticate(w, r, "repositories:push")
	if !ok {
		return
	}
	requestID := newRequestID()
	if err := h.appendAudit(principal.TokenID, "lfs.upload.authorize", repositoryID+":"+oid, requestID, "authorized"); err != nil {
		http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
		return
	}
	lock := &h.locks[stripe(oid)]
	lock.Lock()
	defer lock.Unlock()
	var size int64
	var complete bool
	var err error
	if contentRange := r.Header.Get("Content-Range"); contentRange != "" {
		size, complete, err = h.uploadRange(r, oid, contentRange)
	} else {
		size, err = h.uploadFull(r, oid)
		complete = err == nil
	}
	if err != nil {
		handleUploadError(w, err)
		_ = h.appendAudit(principal.TokenID, "lfs.upload", repositoryID+":"+oid, requestID, "failed")
		return
	}
	if !complete {
		w.Header().Set("X-Gitsss-Upload-Offset", strconv.FormatInt(size, 10))
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.appendAudit(principal.TokenID, "lfs.upload", repositoryID+":"+oid, requestID, "stored"); err != nil {
		http.Error(w, "audit state could not be updated", http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", `"sha256:`+oid+`"`)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) uploadFull(r *http.Request, oid string) (int64, error) {
	size := r.ContentLength
	if size < 0 || size > maximumObjectBytes {
		return 0, errInvalidSize
	}
	if existing, err := regularFileInfo(h.objectPath(oid)); err == nil {
		if existing.Size() == size {
			return size, nil
		}
		return 0, errObjectConflict
	}
	if admission := h.storage.Admit(size); !admission.Allowed {
		return 0, fmt.Errorf("%w: %s", errInsufficientStorage, admissionReason(admission))
	}
	temporary, err := os.CreateTemp(filepath.Join(h.root, ".partial"), "full-*")
	if err != nil {
		return 0, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := state.ProtectFile(temporaryPath); err != nil {
		temporary.Close()
		return 0, err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(r.Body, size+1))
	if copyErr != nil || written != size {
		temporary.Close()
		return written, errInvalidSize
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return written, err
	}
	if err := temporary.Close(); err != nil {
		return written, err
	}
	if subtle.ConstantTimeCompare(hash.Sum(nil), mustDecodeOID(oid)) != 1 {
		return written, errHashMismatch
	}
	if err := h.publish(oid, temporaryPath, size); err != nil {
		return written, err
	}
	_ = os.Remove(h.partialPath(oid))
	return written, nil
}

func (h *Handler) uploadRange(r *http.Request, oid, value string) (int64, bool, error) {
	start, end, total, err := parseContentRange(value)
	if err != nil || total > maximumObjectBytes || r.ContentLength != end-start+1 {
		return 0, false, errInvalidRange
	}
	if existing, err := regularFileInfo(h.objectPath(oid)); err == nil {
		if existing.Size() == total {
			return total, true, nil
		}
		return 0, false, errObjectConflict
	}
	if admission := h.storage.Admit(total - start); !admission.Allowed {
		return 0, false, fmt.Errorf("%w: %s", errInsufficientStorage, admissionReason(admission))
	}
	path := h.partialPath(oid)
	flags := os.O_WRONLY
	if start == 0 {
		flags |= os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, false, errObjectConflict
		}
		return 0, false, err
	}
	defer file.Close()
	if err := state.ProtectFile(path); err != nil {
		return 0, false, err
	}
	info, err := file.Stat()
	if err != nil || info.Size() != start {
		return 0, false, errInvalidRange
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return 0, false, err
	}
	written, err := io.Copy(file, io.LimitReader(r.Body, end-start+2))
	if err != nil || written != end-start+1 {
		_ = file.Truncate(start)
		return start + written, false, errInvalidRange
	}
	if err := file.Sync(); err != nil {
		return start + written, false, err
	}
	offset := start + written
	if offset < total {
		return offset, false, nil
	}
	if err := file.Close(); err != nil {
		return offset, false, err
	}
	valid, err := verifyFileOID(path, oid, total)
	if err != nil {
		return offset, false, err
	}
	if !valid {
		_ = os.Remove(path)
		return offset, false, errHashMismatch
	}
	if err := h.publish(oid, path, total); err != nil {
		return offset, false, err
	}
	return offset, true, nil
}

func (h *Handler) publish(oid, source string, size int64) error {
	target := h.objectPath(oid)
	directory := filepath.Dir(target)
	if err := secureDirectory(directory); err != nil {
		return err
	}
	if err := os.Link(source, target); err != nil {
		if existing, statErr := regularFileInfo(target); statErr == nil && existing.Size() == size {
			return nil
		}
		return err
	}
	if err := state.ProtectFile(target); err != nil {
		_ = os.Remove(target)
		return err
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return nil
}

func (h *Handler) objectAction(r *http.Request, repositoryID, oid string, upload bool) action {
	base := h.publicURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	result := action{Href: base + "/git/" + url.PathEscape(repositoryID) + ".git/info/lfs/objects/" + oid}
	if upload {
		result.Header = map[string]string{"X-Gitsss-Resumable": "sequential-content-range"}
	}
	return result
}

func (h *Handler) objectPath(oid string) string {
	return filepath.Join(h.root, oid[:2], oid[2:4], oid)
}

func (h *Handler) partialPath(oid string) string {
	return filepath.Join(h.root, ".partial", oid)
}

func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request, scope string) (auth.Principal, bool) {
	token, ok := authorizationToken(r)
	if !ok {
		w.Header().Set("LFS-Authenticate", `Basic realm="gitsss"`)
		w.Header().Set("WWW-Authenticate", `Basic realm="gitsss"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "authentication required"})
		return auth.Principal{}, false
	}
	principal, err := h.verifier.Verify(token, scope, h.now().UTC())
	if err == nil {
		return principal, true
	}
	if errors.Is(err, auth.ErrMissingScope) {
		writeJSON(w, http.StatusForbidden, map[string]string{"message": "repository permission denied"})
		return auth.Principal{}, false
	}
	w.Header().Set("LFS-Authenticate", `Basic realm="gitsss"`)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "authentication required"})
	return auth.Principal{}, false
}

func authorizationToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimPrefix(header, "Bearer ")
		return token, token != "" && strings.TrimSpace(token) == token && !strings.ContainsAny(token, " \t\r\n\x00")
	}
	username, password, ok := r.BasicAuth()
	if !ok {
		return "", false
	}
	token := password
	if token == "" {
		token = username
	}
	return token, token != "" && strings.TrimSpace(token) == token && !strings.ContainsAny(token, "\r\n\x00")
}

func parsePath(requestPath string) (string, string, bool) {
	relative := strings.TrimPrefix(requestPath, "/git/")
	marker := ".git/info/lfs/"
	index := strings.Index(relative, marker)
	if relative == requestPath || index < 1 || strings.Contains(relative[:index], "/") {
		return "", "", false
	}
	return relative[:index], relative[index+len(marker):], true
}

func secureDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		return state.ProtectDirectory(path)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("LFS path must be a real directory")
	}
	return state.ProtectDirectory(path)
}

func regularFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("LFS object is not a regular file")
	}
	return info, nil
}

func validOID(oid string) bool {
	decoded, err := hex.DecodeString(oid)
	return err == nil && len(decoded) == sha256.Size && oid == strings.ToLower(oid)
}

func mustDecodeOID(oid string) []byte {
	decoded, _ := hex.DecodeString(oid)
	return decoded
}

func verifyFileOID(path, oid string, size int64) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, size+1))
	if err != nil || written != size {
		return false, errInvalidSize
	}
	return subtle.ConstantTimeCompare(hash.Sum(nil), mustDecodeOID(oid)) == 1, nil
}

func parseContentRange(value string) (int64, int64, int64, error) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, errInvalidRange
	}
	rangeAndTotal := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(rangeAndTotal) != 2 {
		return 0, 0, 0, errInvalidRange
	}
	bounds := strings.Split(rangeAndTotal[0], "-")
	if len(bounds) != 2 {
		return 0, 0, 0, errInvalidRange
	}
	start, startErr := strconv.ParseInt(bounds[0], 10, 64)
	end, endErr := strconv.ParseInt(bounds[1], 10, 64)
	total, totalErr := strconv.ParseInt(rangeAndTotal[1], 10, 64)
	if startErr != nil || endErr != nil || totalErr != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, errInvalidRange
	}
	return start, end, total, nil
}

func (h *Handler) appendAudit(actor, actionName, target, requestID, result string) error {
	return h.audit.Append(audit.Entry{
		Actor: actor, Action: actionName, Target: "repository:" + target,
		RequestID: requestID, Result: result, At: h.now().UTC(),
	})
}

func stripe(oid string) int {
	decoded, _ := hex.DecodeString(oid[:2])
	return int(decoded[0]) % 64
}

func admissionReason(admission storage.Admission) string {
	if strings.TrimSpace(admission.Reason) != "" {
		return admission.Reason
	}
	return "storage write protection is active"
}

func handleUploadError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, errInsufficientStorage):
		status = http.StatusInsufficientStorage
	case errors.Is(err, errObjectConflict):
		status = http.StatusConflict
	case !errors.Is(err, errInvalidSize) && !errors.Is(err, errInvalidRange) && !errors.Is(err, errHashMismatch):
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{"message": err.Error()})
}

func writeLFSJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", mediaType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("lfs-%d", time.Now().UnixNano())
	}
	return "lfs-" + hex.EncodeToString(value)
}

var (
	errInvalidSize         = errors.New("invalid LFS object size")
	errInvalidRange        = errors.New("invalid LFS content range")
	errHashMismatch        = errors.New("LFS object hash mismatch")
	errObjectConflict      = errors.New("LFS object upload conflicts with existing state")
	errInsufficientStorage = errors.New("LFS storage admission denied")
)
