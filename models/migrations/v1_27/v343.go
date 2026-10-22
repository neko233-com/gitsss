// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "github.com/neko233-com/gitsss/models/db"

type orgShareItem struct {
	ID          int64  `xorm:"pk autoincr"`
	OrgID       int64  `xorm:"INDEX NOT NULL"`
	ParentID    int64  `xorm:"INDEX NOT NULL DEFAULT 0"`
	Name        string `xorm:"VARCHAR(255) NOT NULL"`
	Kind        string `xorm:"VARCHAR(16) NOT NULL"`
	Size        int64  `xorm:"NOT NULL DEFAULT 0"`
	SHA256      string `xorm:"VARCHAR(64)"`
	StorageKey  string `xorm:"VARCHAR(255)"`
	Text        string `xorm:"TEXT"`
	UploaderID  int64  `xorm:"INDEX NOT NULL"`
	ExpiresUnix int64  `xorm:"INDEX"`
	CreatedUnix int64  `xorm:"INDEX created"`
	UpdatedUnix int64  `xorm:"INDEX updated"`
}

func (orgShareItem) TableName() string { return "org_share_item" }

type orgShareUpload struct {
	ID           string `xorm:"pk VARCHAR(64)"`
	OrgID        int64  `xorm:"INDEX NOT NULL"`
	ParentID     int64  `xorm:"NOT NULL DEFAULT 0"`
	Name         string `xorm:"VARCHAR(255) NOT NULL"`
	Size         int64  `xorm:"NOT NULL"`
	Received     int64  `xorm:"NOT NULL DEFAULT 0"`
	SHA256       string `xorm:"VARCHAR(64)"`
	Ranges       string `xorm:"ranges_json TEXT"`
	Preallocated bool   `xorm:"NOT NULL DEFAULT false"`
	UploaderID   int64  `xorm:"NOT NULL"`
	ExpiresUnix  int64  `xorm:"INDEX"`
	CreatedUnix  int64  `xorm:"created"`
}

func (orgShareUpload) TableName() string { return "org_share_upload" }

type orgShareQuota struct {
	OrgID       int64 `xorm:"pk"`
	MaxBytes    int64 `xorm:"NOT NULL"`
	UpdatedUnix int64 `xorm:"updated"`
}

func (orgShareQuota) TableName() string { return "org_share_quota" }

// AddOrgShareTables creates the organization file-share tables.
func AddOrgShareTables(x db.EngineMigration) error {
	return x.Sync(new(orgShareItem), new(orgShareUpload), new(orgShareQuota))
}
