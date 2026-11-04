// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"

	"github.com/neko233-com/gitsss/models/db"
	system_model "github.com/neko233-com/gitsss/models/system"
	"github.com/neko233-com/gitsss/modules/json"
	"github.com/neko233-com/gitsss/modules/util"
)

// Revision reads only the revision, avoiding transfer of large state values.
func (f *DBStore) Revision(ctx context.Context, name string) (int64, bool, error) {
	row := &system_model.AppState{}
	has, err := db.GetEngine(ctx).Select("revision").ID(name).Get(row)
	return row.Revision, has, err
}

// GetVersioned reads a state value and its revision in one database query.
func (f *DBStore) GetVersioned(ctx context.Context, item StateItem) (int64, bool, error) {
	row := &system_model.AppState{}
	has, err := db.GetEngine(ctx).ID(item.Name()).Get(row)
	if err != nil || !has || row.Content == "" {
		return row.Revision, has, err
	}
	return row.Revision, true, json.Unmarshal(util.UnsafeStringToBytes(row.Content), item)
}

// CompareAndSet writes only if the caller's database revision is still current.
func (f *DBStore) CompareAndSet(ctx context.Context, item StateItem, revision int64, exists bool) (bool, error) {
	content, err := json.Marshal(item)
	if err != nil {
		return false, err
	}
	if !exists {
		err = db.Insert(ctx, &system_model.AppState{ID: item.Name(), Content: util.UnsafeBytesToString(content), Revision: 1})
		if err == nil {
			return true, nil
		}
		_, nowExists, checkErr := f.Revision(ctx, item.Name())
		if checkErr == nil && nowExists {
			return false, nil
		}
		return false, err
	}
	changed, err := db.GetEngine(ctx).Where("id=? AND revision=?", item.Name(), revision).
		Cols("content", "revision").Update(&system_model.AppState{Content: util.UnsafeBytesToString(content), Revision: revision + 1})
	return changed == 1, err
}
