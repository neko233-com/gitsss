// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/neko233-com/gitsss/modules/json"
)

// DeliveryStore records successfully delivered cards for a report date.
type DeliveryStore interface {
	Delivered(date, account string) (bool, error)
	MarkDelivered(date, account string) error
}

// RunResult contains counts only; commit messages and credentials stay private.
type RunResult struct {
	Sent    int
	Skipped int
	Failed  int
}

// Run delivers one card for each classified author with commits.
// Callers must serialize runs that share a delivery store.
func Run(ctx context.Context, date string, grouped map[string][]AttributedCommit, store DeliveryStore, summarize func(context.Context, string) (string, error), send func(context.Context, string, string) error) (RunResult, error) {
	var result RunResult
	if date == "" || store == nil || summarize == nil || send == nil {
		return result, errors.New("incomplete daily report runner")
	}
	accounts := make([]string, 0, len(grouped))
	var failures []error
	for account, commits := range grouped {
		if account != "" && len(commits) > 0 {
			accounts = append(accounts, account)
		}
	}
	sort.Strings(accounts)
	for _, account := range accounts {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		delivered, err := store.Delivered(date, account)
		if err != nil {
			return result, errors.New("cannot read daily report delivery state")
		}
		if delivered {
			result.Skipped++
			continue
		}
		input, err := json.Marshal(grouped[account])
		if err != nil {
			return result, errors.New("cannot prepare daily report input")
		}
		summary, err := summarize(ctx, string(input))
		if err != nil {
			err = fmt.Errorf("summarize: %w", err)
		} else if err = send(ctx, account, summary); err != nil {
			err = fmt.Errorf("send card: %w", err)
		}
		if err != nil {
			result.Failed++
			failures = append(failures, err)
			continue
		}
		if err := store.MarkDelivered(date, account); err != nil {
			return result, errors.New("card was sent but delivery state could not be saved")
		}
		result.Sent++
	}
	if result.Failed != 0 {
		return result, errors.Join(fmt.Errorf("%d daily report cards failed", result.Failed), errors.Join(failures...))
	}
	return result, nil
}
