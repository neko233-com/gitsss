// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"
	"time"
)

// CronTask represents a Cron task
type CronTask struct {
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Next      time.Time `json:"next"`
	Prev      time.Time `json:"prev"`
	ExecTimes int64     `json:"exec_times"`
}

// ListCronTaskOptions list options for ListCronTasks
type ListCronTaskOptions struct {
	ListOptions
}

// ListCronTasks list available cron tasks
func (c *AdminService) ListCronTasks(ctx context.Context, opt ListCronTaskOptions) ([]*CronTask, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_13_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	ct := make([]*CronTask, 0, opt.PageSize)
	resp, err := c.getParsedResponse(ctx, "GET", fmt.Sprintf("/admin/cron?%s", opt.getURLQuery().Encode()), jsonHeader, nil, &ct)
	return ct, resp, err
}

// RunCronTasks run a cron task
func (c *AdminService) RunCronTasks(ctx context.Context, task string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(ctx, version1_13_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&task); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle(ctx, "POST", fmt.Sprintf("/admin/cron/%s", task), jsonHeader, nil)
}
