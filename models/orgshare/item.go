// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package orgshare

import (
	"context"
	"fmt"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/timeutil"
	"github.com/neko233-com/gitsss/modules/util"
)

func init() {
	db.RegisterModel(new(ShareItem))
	db.RegisterModel(new(ShareUpload))
	db.RegisterModel(new(ShareQuota))
}

const (
	KindFile = "file"
	KindDir  = "dir"
	KindText = "text"
)

// ShareItem is one file, folder, or text note in an organization share.
type ShareItem struct {
	ID          int64              `xorm:"pk autoincr"`
	OrgID       int64              `xorm:"INDEX NOT NULL"`
	ParentID    int64              `xorm:"INDEX NOT NULL DEFAULT 0"`
	Name        string             `xorm:"VARCHAR(255) NOT NULL"`
	Kind        string             `xorm:"VARCHAR(16) NOT NULL"`
	Size        int64              `xorm:"NOT NULL DEFAULT 0"`
	SHA256      string             `xorm:"VARCHAR(64)"`
	StorageKey  string             `xorm:"VARCHAR(255)"`
	Text        string             `xorm:"TEXT"`
	UploaderID  int64              `xorm:"INDEX NOT NULL"`
	ExpiresUnix timeutil.TimeStamp `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName keeps the table out of Git and LFS tables.
func (ShareItem) TableName() string { return "org_share_item" }

// ShareUpload is an in-progress resumable upload.
type ShareUpload struct {
	ID           string             `xorm:"pk VARCHAR(64)"`
	OrgID        int64              `xorm:"INDEX NOT NULL"`
	ParentID     int64              `xorm:"NOT NULL DEFAULT 0"`
	Name         string             `xorm:"VARCHAR(255) NOT NULL"`
	Size         int64              `xorm:"NOT NULL"`
	Received     int64              `xorm:"NOT NULL DEFAULT 0"`
	SHA256       string             `xorm:"VARCHAR(64)"`
	Ranges       string             `xorm:"ranges_json TEXT"`
	Preallocated bool               `xorm:"NOT NULL DEFAULT false"`
	UploaderID   int64              `xorm:"NOT NULL"`
	ExpiresUnix  timeutil.TimeStamp `xorm:"INDEX"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

// TableName keeps upload sessions beside share items.
func (ShareUpload) TableName() string { return "org_share_upload" }

// ShareQuota is an optional per-organization cap. A missing row uses the server default.
type ShareQuota struct {
	OrgID       int64              `xorm:"pk"`
	MaxBytes    int64              `xorm:"NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// TableName keeps quota rows beside share items.
func (ShareQuota) TableName() string { return "org_share_quota" }

// ErrNotExist is returned when a share object is missing.
type ErrNotExist struct{ ID int64 }

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("share item %d does not exist", err.ID)
}

func (err ErrNotExist) Unwrap() error { return util.ErrNotExist }

// IsErrNotExist reports a missing share item.
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

// InsertItem stores a new share object.
func InsertItem(ctx context.Context, item *ShareItem) error {
	_, err := db.GetEngine(ctx).Insert(item)
	return err
}

// GetItem loads one object in an organization.
func GetItem(ctx context.Context, orgID, id int64) (*ShareItem, error) {
	item := &ShareItem{ID: id, OrgID: orgID}
	has, err := db.GetEngine(ctx).Get(item)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{ID: id}
	}
	return item, nil
}

// FindChild looks up one named child.
func FindChild(ctx context.Context, orgID, parentID int64, name, kind string) (*ShareItem, error) {
	item := &ShareItem{OrgID: orgID, ParentID: parentID, Name: name, Kind: kind}
	has, err := db.GetEngine(ctx).Get(item)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{}
	}
	return item, nil
}

// ListItems returns the children of a folder. parentID 0 is the root.
func ListItems(ctx context.Context, orgID, parentID int64) ([]*ShareItem, error) {
	items := make([]*ShareItem, 0)
	err := db.GetEngine(ctx).Where("org_id=? AND parent_id=?", orgID, parentID).OrderBy("kind, name").Find(&items)
	return items, err
}

// ListExpired returns objects whose deadline has passed.
func ListExpired(ctx context.Context, now timeutil.TimeStamp) ([]*ShareItem, error) {
	items := make([]*ShareItem, 0)
	err := db.GetEngine(ctx).Where("expires_unix>0 AND expires_unix<=?", now).Find(&items)
	return items, err
}

// UpdateStorageKey stores the object path after the row id exists.
func UpdateStorageKey(ctx context.Context, item *ShareItem) error {
	_, err := db.GetEngine(ctx).ID(item.ID).Cols("storage_key").Update(item)
	return err
}

// UpdateExpiry stores a new deadline. Zero means the object is permanent.
func UpdateExpiry(ctx context.Context, item *ShareItem) error {
	_, err := db.GetEngine(ctx).ID(item.ID).Cols("expires_unix").Update(item)
	return err
}

// DeleteItem removes one row.
func DeleteItem(ctx context.Context, orgID, id int64) error {
	_, err := db.GetEngine(ctx).Where("org_id=? AND id=?", orgID, id).Delete(&ShareItem{})
	return err
}

// UsedBytes sums stored object sizes for an organization.
func UsedBytes(ctx context.Context, orgID int64) (int64, error) {
	return db.GetEngine(ctx).Where("org_id=?", orgID).SumInt(&ShareItem{}, "size")
}

// ReservedBytes is stored objects plus in-progress uploads.
func ReservedBytes(ctx context.Context, orgID int64) (int64, error) {
	items, err := UsedBytes(ctx, orgID)
	if err != nil {
		return 0, err
	}
	uploads, err := db.GetEngine(ctx).Where("org_id=?", orgID).SumInt(&ShareUpload{}, "size")
	if err != nil {
		return 0, err
	}
	return items + uploads, nil
}

// CountFiles counts files and text notes, not folders.
func CountFiles(ctx context.Context, orgID int64) (int64, error) {
	return db.GetEngine(ctx).Where("org_id=? AND kind<>?", orgID, KindDir).Count(&ShareItem{})
}

// InsertUpload stores a resumable session.
func InsertUpload(ctx context.Context, upload *ShareUpload) error {
	_, err := db.GetEngine(ctx).Insert(upload)
	return err
}

// GetUpload loads a session owned by the organization.
func GetUpload(ctx context.Context, orgID int64, id string) (*ShareUpload, error) {
	upload := &ShareUpload{ID: id, OrgID: orgID}
	has, err := db.GetEngine(ctx).Get(upload)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{}
	}
	return upload, nil
}

// FindUpload returns an in-progress session with the same name and size.
func FindUpload(ctx context.Context, orgID, parentID, uploaderID, size int64, name string) (*ShareUpload, error) {
	upload := &ShareUpload{}
	has, err := db.GetEngine(ctx).
		Where("org_id=? AND parent_id=? AND uploader_id=? AND size=? AND name=?", orgID, parentID, uploaderID, size, name).
		Get(upload)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{}
	}
	return upload, nil
}

// UpdateUploadProgress stores covered ranges after a chunk lands.
func UpdateUploadProgress(ctx context.Context, upload *ShareUpload) error {
	_, err := db.GetEngine(ctx).ID(upload.ID).Cols("received", "ranges_json").Update(upload)
	return err
}

// DeleteUpload removes a session.
func DeleteUpload(ctx context.Context, id string) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&ShareUpload{})
	return err
}

// ListExpiredUploads returns sessions whose deadline has passed.
func ListExpiredUploads(ctx context.Context, now timeutil.TimeStamp) ([]*ShareUpload, error) {
	uploads := make([]*ShareUpload, 0)
	err := db.GetEngine(ctx).Where("expires_unix>0 AND expires_unix<=?", now).Find(&uploads)
	return uploads, err
}

// GetQuota loads a per-organization cap.
func GetQuota(ctx context.Context, orgID int64) (*ShareQuota, error) {
	row := &ShareQuota{OrgID: orgID}
	has, err := db.GetEngine(ctx).Get(row)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrNotExist{}
	}
	return row, nil
}

// SetQuota stores a per-organization cap. Zero deletes the override and the default applies.
func SetQuota(ctx context.Context, orgID, maxBytes int64) error {
	if maxBytes <= 0 {
		_, err := db.GetEngine(ctx).ID(orgID).Delete(&ShareQuota{})
		return err
	}
	_, err := GetQuota(ctx, orgID)
	if IsErrNotExist(err) {
		_, err = db.GetEngine(ctx).Insert(&ShareQuota{OrgID: orgID, MaxBytes: maxBytes})
		return err
	}
	if err != nil {
		return err
	}
	_, err = db.GetEngine(ctx).ID(orgID).Cols("max_bytes").Update(&ShareQuota{MaxBytes: maxBytes})
	return err
}
