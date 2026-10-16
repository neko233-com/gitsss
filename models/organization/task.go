// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/neko233-com/gitsss/models/db"
	"github.com/neko233-com/gitsss/modules/timeutil"

	"xorm.io/builder"
)

// OrgTask is a standalone card on an organization's single shared board.
type OrgTask struct {
	ID          int64  `xorm:"pk autoincr"`
	OrgID       int64  `xorm:"INDEX NOT NULL"`
	Title       string `xorm:"VARCHAR(255) NOT NULL"`
	Content     string `xorm:"TEXT"`
	Status      string `xorm:"VARCHAR(16) NOT NULL"`
	AssigneeID  int64  `xorm:"INDEX"`
	CreatorID   int64
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(OrgTask))
}

var ErrInvalidOrgTask = errors.New("invalid organization task")

func ValidOrgTaskStatus(status string) bool {
	return status == "todo" || status == "doing" || status == "done"
}

func ValidateOrgTask(ctx context.Context, task *OrgTask) error {
	task.Title = strings.TrimSpace(task.Title)
	if task.OrgID <= 0 || task.Title == "" || utf8.RuneCountInString(task.Title) > 255 || utf8.RuneCountInString(task.Content) > 10000 || !ValidOrgTaskStatus(task.Status) || task.AssigneeID < 0 {
		return ErrInvalidOrgTask
	}
	if task.AssigneeID != 0 {
		member, err := IsOrganizationMember(ctx, task.OrgID, task.AssigneeID)
		if err != nil {
			return err
		}
		if !member {
			return ErrInvalidOrgTask
		}
	}
	return nil
}

func CreateOrgTask(ctx context.Context, task *OrgTask) error {
	if err := ValidateOrgTask(ctx, task); err != nil {
		return err
	}
	_, err := db.GetEngine(ctx).Insert(task)
	return err
}

func GetOrgTask(ctx context.Context, orgID, taskID int64) (*OrgTask, bool, error) {
	return db.Get[OrgTask](ctx, builder.Eq{"id": taskID, "org_id": orgID})
}

func UpdateOrgTask(ctx context.Context, task *OrgTask) (bool, error) {
	if err := ValidateOrgTask(ctx, task); err != nil {
		return false, err
	}
	n, err := db.GetEngine(ctx).Where("id = ? AND org_id = ?", task.ID, task.OrgID).
		Cols("title", "content", "status", "assignee_id").Update(task)
	return n > 0, err
}

func MoveOrgTask(ctx context.Context, orgID, taskID int64, status string) (bool, error) {
	if !ValidOrgTaskStatus(status) {
		return false, ErrInvalidOrgTask
	}
	n, err := db.GetEngine(ctx).Where("id = ? AND org_id = ?", taskID, orgID).
		Cols("status").Update(&OrgTask{Status: status})
	return n > 0, err
}

func DeleteOrgTask(ctx context.Context, orgID, taskID int64) error {
	_, err := db.GetEngine(ctx).Where("id = ? AND org_id = ?", taskID, orgID).Delete(new(OrgTask))
	return err
}

// FindOrgTasks bounds rendering cost; the total lets the UI request a narrower search.
func FindOrgTasks(ctx context.Context, orgID int64, query string, assigneeID int64) ([]*OrgTask, int64, error) {
	cond := builder.Eq{"org_id": orgID}.And(builder.NewCond())
	if query = strings.TrimSpace(query); query != "" {
		cond = cond.And(builder.Or(db.BuildCaseInsensitiveLike("title", query), db.BuildCaseInsensitiveLike("content", query)))
	}
	if assigneeID > 0 {
		cond = cond.And(builder.Eq{"assignee_id": assigneeID})
	}
	total, err := db.GetEngine(ctx).Where(cond).Count(new(OrgTask))
	if err != nil {
		return nil, 0, err
	}
	tasks := make([]*OrgTask, 0)
	err = db.GetEngine(ctx).Where(cond).Desc("updated_unix", "id").Limit(500).Find(&tasks)
	return tasks, total, err
}
