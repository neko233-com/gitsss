// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	share_model "github.com/neko233-com/gitsss/models/orgshare"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/orgshare"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/timeutil"
)

var (
	uploadLocks sync.Map
	orgLocks    sync.Map
)

// Root is the organization share directory. It is not a Git or LFS path.
func Root() string {
	if setting.OrgShare.Path != "" {
		return setting.OrgShare.Path
	}
	return filepath.Join(setting.AppDataPath, "org-share")
}

func objectPath(orgID, itemID int64) string {
	return filepath.Join(Root(), "objects", strconv.FormatInt(orgID, 10), strconv.FormatInt(itemID, 10))
}

func uploadPath(id string) string {
	return filepath.Join(Root(), "uploads", id)
}

// Usage is one organization row on the admin page.
type Usage struct {
	OrgID    int64  `json:"org_id"`
	Name     string `json:"name"`
	Used     int64  `json:"used_bytes"`
	Quota    int64  `json:"quota_bytes"`
	Files    int64  `json:"files"`
	Override bool   `json:"override"`
}

// DiskReport is the share volume plus each organization.
type DiskReport struct {
	Path      string  `json:"path"`
	Capacity  int64   `json:"capacity_bytes"`
	Available int64   `json:"available_bytes"`
	Reserve   int64   `json:"reserve_bytes"`
	Error     string  `json:"error,omitempty"`
	Orgs      []Usage `json:"orgs"`
}

// List returns one folder after deleting expired objects.
func List(ctx context.Context, orgID, parentID int64) ([]*share_model.ShareItem, error) {
	if err := Cleanup(ctx, time.Now()); err != nil {
		return nil, err
	}
	return share_model.ListItems(ctx, orgID, parentID)
}

// Ancestors walks from a folder up to the root.
func Ancestors(ctx context.Context, orgID, parentID int64) ([]*share_model.ShareItem, error) {
	chain := make([]*share_model.ShareItem, 0)
	seen := map[int64]bool{}
	for parentID != 0 && len(chain) < 32 && !seen[parentID] {
		seen[parentID] = true
		item, err := share_model.GetItem(ctx, orgID, parentID)
		if err != nil {
			return nil, err
		}
		chain = append(chain, item)
		parentID = item.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// CreateText stores a shared note.
func CreateText(ctx context.Context, orgID, uploaderID, parentID int64, name, body string, permanent bool) (*share_model.ShareItem, error) {
	parts, err := orgshare.SafeParts(name)
	if err != nil || len(parts) != 1 {
		return nil, errors.New("text name must be a single path segment")
	}
	if err := requireParent(ctx, orgID, parentID); err != nil {
		return nil, err
	}
	unlock := lockOrg(orgID)
	defer unlock()
	if err := admit(ctx, orgID, int64(len(body))); err != nil {
		return nil, err
	}
	item := &share_model.ShareItem{
		OrgID: orgID, ParentID: parentID, Name: parts[0], Kind: share_model.KindText,
		Size: int64(len(body)), Text: body, UploaderID: uploaderID, ExpiresUnix: expiry(permanent),
	}
	if err := share_model.InsertItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// BeginUpload creates parent folders and a resumable session.
// resume returns an existing session for the same object instead of reserving the quota twice.
func BeginUpload(ctx context.Context, orgID, uploaderID, parentID int64, relativePath string, size int64, permanent, resume bool) (*share_model.ShareUpload, error) {
	parts, err := orgshare.SafeParts(relativePath)
	if err != nil {
		return nil, err
	}
	if err := requireParent(ctx, orgID, parentID); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, errors.New("upload size must not be negative")
	}
	unlock := lockOrg(orgID)
	defer unlock()
	for _, dir := range parts[:len(parts)-1] {
		parentID, err = ensureDir(ctx, orgID, uploaderID, parentID, dir)
		if err != nil {
			return nil, err
		}
	}
	name := parts[len(parts)-1]
	if resume {
		existing, findErr := share_model.FindUpload(ctx, orgID, parentID, uploaderID, size, name)
		if findErr == nil {
			return existing, nil
		}
		if !share_model.IsErrNotExist(findErr) {
			return nil, findErr
		}
	}
	if err := admit(ctx, orgID, size); err != nil {
		return nil, err
	}
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(uploadPath(id)), 0o750); err != nil {
		return nil, err
	}
	preallocated, err := orgshare.Reserve(uploadPath(id), size)
	if err != nil {
		_ = os.Remove(uploadPath(id))
		return nil, diskFull(size, err)
	}
	upload := &share_model.ShareUpload{
		ID: id, OrgID: orgID, ParentID: parentID, Name: name,
		Size: size, UploaderID: uploaderID, ExpiresUnix: expiry(permanent), Preallocated: preallocated,
	}
	if err := share_model.InsertUpload(ctx, upload); err != nil {
		_ = os.Remove(uploadPath(id))
		return nil, err
	}
	return upload, nil
}

// Append writes one chunk. Chunks may arrive out of order.
func Append(ctx context.Context, orgID int64, uploadID string, start int64, r io.Reader, length int64) (*share_model.ShareUpload, error) {
	unlock := lockUpload(uploadID)
	defer unlock()
	upload, err := share_model.GetUpload(ctx, orgID, uploadID)
	if err != nil {
		return nil, err
	}
	if start < 0 || length < 0 || start+length > upload.Size {
		return nil, fmt.Errorf("chunk %d+%d is outside %d", start, length, upload.Size)
	}
	spans, err := decodeSpans(upload.Ranges)
	if err != nil {
		return nil, err
	}
	if length == 0 || coveredSpan(spans, start, start+length) {
		return upload, nil
	}
	if !upload.Preallocated {
		if err := admitDisk(length); err != nil {
			return nil, err
		}
	}
	if err := orgshare.WriteAtReader(uploadPath(upload.ID), start, r, length); err != nil {
		return nil, err
	}
	spans, err = orgshare.AddSpan(spans, start, start+length)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(spans)
	if err != nil {
		return nil, err
	}
	upload.Ranges = string(encoded)
	upload.Received = orgshare.CoveredBytes(spans)
	if err := share_model.UpdateUploadProgress(ctx, upload); err != nil {
		return nil, err
	}
	return upload, nil
}

// Complete moves a finished upload into the share store.
func Complete(ctx context.Context, orgID int64, uploadID string) (*share_model.ShareItem, error) {
	unlock := lockUpload(uploadID)
	defer unlock()
	upload, err := share_model.GetUpload(ctx, orgID, uploadID)
	if err != nil {
		return nil, err
	}
	spans, err := decodeSpans(upload.Ranges)
	if err != nil {
		return nil, err
	}
	if len(orgshare.Missing(spans, upload.Size)) != 0 {
		return nil, fmt.Errorf("upload received %d of %d bytes", upload.Received, upload.Size)
	}
	item := &share_model.ShareItem{
		OrgID: orgID, ParentID: upload.ParentID, Name: upload.Name, Kind: share_model.KindFile,
		Size: upload.Size, UploaderID: upload.UploaderID, ExpiresUnix: upload.ExpiresUnix,
	}
	if err := share_model.InsertItem(ctx, item); err != nil {
		return nil, err
	}
	item.StorageKey = objectPath(orgID, item.ID)
	if err := os.MkdirAll(filepath.Dir(item.StorageKey), 0o750); err != nil {
		return nil, err
	}
	if err := os.Rename(uploadPath(upload.ID), item.StorageKey); err != nil {
		_ = share_model.DeleteItem(ctx, orgID, item.ID)
		return nil, err
	}
	if err := share_model.UpdateStorageKey(ctx, item); err != nil {
		return nil, err
	}
	if err := share_model.DeleteUpload(ctx, upload.ID); err != nil {
		return nil, err
	}
	return item, nil
}

// Open returns a file object for streaming download.
func Open(ctx context.Context, orgID, id int64) (*share_model.ShareItem, *os.File, error) {
	item, err := share_model.GetItem(ctx, orgID, id)
	if err != nil {
		return nil, nil, err
	}
	if item.Kind != share_model.KindFile {
		return item, nil, nil
	}
	file, err := orgshare.Open(item.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return item, file, nil
}

// SetPermanent changes one object's deadline. permanent false applies the configured lifetime from now.
func SetPermanent(ctx context.Context, orgID, id int64, permanent bool) (*share_model.ShareItem, error) {
	item, err := share_model.GetItem(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	item.ExpiresUnix = expiry(permanent)
	if err := share_model.UpdateExpiry(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// Delete removes one object. Directories remove their children first.
func Delete(ctx context.Context, orgID, id int64) error {
	item, err := share_model.GetItem(ctx, orgID, id)
	if err != nil {
		return err
	}
	return deleteTree(ctx, item)
}

// Clear removes every object in the organization share.
func Clear(ctx context.Context, orgID int64) error {
	items, err := share_model.ListItems(ctx, orgID, 0)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := deleteTree(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// Cleanup deletes expired objects and abandoned uploads.
func Cleanup(ctx context.Context, now time.Time) error {
	stamp := timeutil.TimeStamp(now.Unix())
	items, err := share_model.ListExpired(ctx, stamp)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := deleteTree(ctx, item); err != nil {
			return err
		}
	}
	uploads, err := share_model.ListExpiredUploads(ctx, stamp)
	if err != nil {
		return err
	}
	for _, upload := range uploads {
		_ = os.Remove(uploadPath(upload.ID))
		if err := share_model.DeleteUpload(ctx, upload.ID); err != nil {
			return err
		}
	}
	return nil
}

// QuotaLimit is the per-organization cap, or the server default when no override is stored.
func QuotaLimit(ctx context.Context, orgID int64) (int64, bool, error) {
	row, err := share_model.GetQuota(ctx, orgID)
	if share_model.IsErrNotExist(err) {
		return setting.OrgShare.MaxBytes, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if row.MaxBytes <= 0 {
		return setting.OrgShare.MaxBytes, false, nil
	}
	return row.MaxBytes, true, nil
}

// SetQuota stores a site-admin override. Non-positive maxBytes restores the default.
func SetQuota(ctx context.Context, orgID, maxBytes int64) error {
	return share_model.SetQuota(ctx, orgID, maxBytes)
}

// Report reads the share volume and every organization that has a name.
func Report(ctx context.Context, orgs []Usage) (*DiskReport, error) {
	if err := os.MkdirAll(Root(), 0o750); err != nil {
		return nil, err
	}
	report := &DiskReport{Path: Root(), Reserve: setting.OrgShare.ReserveBytes, Orgs: orgs}
	volume, err := orgshare.MeasureVolume(Root())
	if err != nil {
		report.Error = err.Error()
		return report, nil
	}
	report.Capacity = volume.CapacityBytes
	report.Available = volume.AvailableBytes
	return report, nil
}

// FillUsage loads used bytes and the effective quota for one organization.
func FillUsage(ctx context.Context, orgID int64, name string) (Usage, error) {
	used, err := share_model.ReservedBytes(ctx, orgID)
	if err != nil {
		return Usage{}, err
	}
	files, err := share_model.CountFiles(ctx, orgID)
	if err != nil {
		return Usage{}, err
	}
	quota, override, err := QuotaLimit(ctx, orgID)
	if err != nil {
		return Usage{}, err
	}
	return Usage{OrgID: orgID, Name: name, Used: used, Quota: quota, Files: files, Override: override}, nil
}

// UploadMissing returns byte ranges that still need to be sent.
func UploadMissing(upload *share_model.ShareUpload) ([]orgshare.Span, error) {
	spans, err := decodeSpans(upload.Ranges)
	if err != nil {
		return nil, err
	}
	missing := orgshare.Missing(spans, upload.Size)
	if missing == nil {
		missing = []orgshare.Span{}
	}
	return missing, nil
}

func deleteTree(ctx context.Context, item *share_model.ShareItem) error {
	if item.Kind == share_model.KindDir {
		children, err := share_model.ListItems(ctx, item.OrgID, item.ID)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := deleteTree(ctx, child); err != nil {
				return err
			}
		}
	}
	if item.StorageKey != "" {
		_ = os.Remove(item.StorageKey)
	}
	return share_model.DeleteItem(ctx, item.OrgID, item.ID)
}

func ensureDir(ctx context.Context, orgID, uploaderID, parentID int64, name string) (int64, error) {
	existing, err := share_model.FindChild(ctx, orgID, parentID, name, share_model.KindDir)
	if err == nil {
		return existing.ID, nil
	}
	if !share_model.IsErrNotExist(err) {
		return 0, err
	}
	item := &share_model.ShareItem{
		OrgID: orgID, ParentID: parentID, Name: name, Kind: share_model.KindDir, UploaderID: uploaderID,
	}
	if err := share_model.InsertItem(ctx, item); err != nil {
		return 0, err
	}
	return item.ID, nil
}

func requireParent(ctx context.Context, orgID, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	item, err := share_model.GetItem(ctx, orgID, parentID)
	if err != nil {
		return err
	}
	if item.Kind != share_model.KindDir {
		return errors.New("parent is not a folder")
	}
	return nil
}

func admit(ctx context.Context, orgID, incoming int64) error {
	if err := os.MkdirAll(Root(), 0o750); err != nil {
		return err
	}
	used, err := share_model.ReservedBytes(ctx, orgID)
	if err != nil {
		return err
	}
	limit, _, err := QuotaLimit(ctx, orgID)
	if err != nil {
		return err
	}
	volume, measureErr := orgshare.MeasureVolume(Root())
	return orgshare.Admit(orgshare.AdmitInput{
		Used: used, Incoming: incoming, MaxBytes: limit, MaxFile: setting.OrgShare.MaxFileBytes,
		Available: volume.AvailableBytes, Reserve: setting.OrgShare.ReserveBytes, DiskKnown: measureErr == nil,
	})
}

func admitDisk(incoming int64) error {
	volume, err := orgshare.MeasureVolume(Root())
	if err != nil {
		return &orgshare.LimitError{Code: "disk_unknown", Incoming: incoming}
	}
	return orgshare.Admit(orgshare.AdmitInput{
		Incoming: incoming, Available: volume.AvailableBytes, Reserve: setting.OrgShare.ReserveBytes, DiskKnown: true,
	})
}

func diskFull(size int64, cause error) error {
	volume, err := orgshare.MeasureVolume(Root())
	if err != nil {
		return fmt.Errorf("磁盘空间不足，无法为本次上传预留 %d 字节: %w", size, cause)
	}
	return &orgshare.LimitError{Code: "disk", Incoming: size, Available: volume.AvailableBytes, Reserve: setting.OrgShare.ReserveBytes}
}

func expiry(permanent bool) timeutil.TimeStamp {
	deadline := orgshare.Deadline(time.Now(), setting.OrgShare.ExpireDays, permanent)
	if deadline.IsZero() {
		return 0
	}
	return timeutil.TimeStamp(deadline.Unix())
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func lockOrg(orgID int64) func() {
	value, _ := orgLocks.LoadOrStore(orgID, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func lockUpload(id string) func() {
	value, _ := uploadLocks.LoadOrStore(id, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func decodeSpans(raw string) ([]orgshare.Span, error) {
	if raw == "" {
		return nil, nil
	}
	var spans []orgshare.Span
	if err := json.Unmarshal([]byte(raw), &spans); err != nil {
		return nil, err
	}
	return spans, nil
}

func coveredSpan(spans []orgshare.Span, start, end int64) bool {
	for _, span := range spans {
		if span.Start <= start && span.End >= end {
			return true
		}
	}
	return false
}
