// Copyright 2026 The Gitsss Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	actions_model "github.com/neko233-com/gitsss/models/actions"
	"github.com/neko233-com/gitsss/models/db"
	repo_model "github.com/neko233-com/gitsss/models/repo"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/timeutil"
)

const dashboardWorkerLimit = 50

// DashboardWorkerItem is an active or queued Actions job shown on the user dashboard.
// Completed jobs are intentionally excluded so the panel remains an operations view.
type DashboardWorkerItem struct {
	Runner *actions_model.ActionRunner
	Job    *actions_model.ActionRunJob
	Run    *actions_model.ActionRun
	Repo   *repo_model.Repository

	Status    actions_model.Status
	IsQueued  bool
	IsRunning bool
	Search    string
	Updated   timeutil.TimeStamp
}

// DashboardStats is the small, read-only Actions operations snapshot used by the home page.
type DashboardStats struct {
	Items []*DashboardWorkerItem

	TotalRunners    int64
	ActiveRunners   int64
	IdleRunners     int64
	OfflineRunners  int64
	DisabledRunners int64
	RunningJobs     int64
	QueuedJobs      int64
	SuccessLastDay  int64
	FailureLastDay  int64
	LastCompleted   *actions_model.ActionRun
}

// GetDashboardStats returns a bounded, database-backed snapshot of runner activity.
// It deliberately keeps the query surface small and does not add a queue or metrics dependency.
func GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}
	if !setting.Actions.Enabled {
		return stats, nil
	}

	if err := populateRunnerStats(ctx, stats, time.Now()); err != nil {
		return nil, err
	}
	runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{
		PageSize: dashboardWorkerLimit,
		Sort:     "online",
	})
	if err != nil {
		return nil, fmt.Errorf("find action runners: %w", err)
	}
	runnerByID := make(map[int64]*actions_model.ActionRunner, len(runners))
	for _, runner := range runners {
		runnerByID[runner.ID] = runner
	}

	var runningJobs, queuedJobs []*actions_model.ActionRunJob
	if err := findDashboardJobs(ctx, &runningJobs, actions_model.StatusRunning); err != nil {
		return nil, err
	}
	if err := findDashboardJobs(ctx, &queuedJobs, actions_model.StatusWaiting, actions_model.StatusBlocked); err != nil {
		return nil, err
	}
	runningJobs = append(runningJobs, queuedJobs...)
	jobs := runningJobs
	stats.RunningJobs, err = countDashboardJobs(ctx, actions_model.StatusRunning)
	if err != nil {
		return nil, err
	}
	stats.QueuedJobs, err = countDashboardJobs(ctx, actions_model.StatusWaiting, actions_model.StatusBlocked)
	if err != nil {
		return nil, err
	}

	var tasks []*actions_model.ActionTask
	if err := db.GetEngine(ctx).
		Where("status = ?", actions_model.StatusRunning).
		Desc("started").
		Limit(dashboardWorkerLimit).
		Find(&tasks); err != nil {
		return nil, fmt.Errorf("find running action tasks: %w", err)
	}
	taskByJobID := make(map[int64]*actions_model.ActionTask, len(tasks))
	taskRunnerIDs := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		taskByJobID[task.JobID] = task
		taskRunnerIDs = append(taskRunnerIDs, task.RunnerID)
	}
	missingRunnerIDs := make([]int64, 0, len(taskRunnerIDs))
	for _, runnerID := range uniquePositiveIDs(taskRunnerIDs) {
		if _, ok := runnerByID[runnerID]; !ok {
			missingRunnerIDs = append(missingRunnerIDs, runnerID)
		}
	}
	if len(missingRunnerIDs) > 0 {
		var assignedRunners []*actions_model.ActionRunner
		if err := db.GetEngine(ctx).In("id", missingRunnerIDs).Find(&assignedRunners); err != nil {
			return nil, fmt.Errorf("find assigned action runners: %w", err)
		}
		for _, runner := range assignedRunners {
			runnerByID[runner.ID] = runner
		}
	}

	runIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		runIDs = append(runIDs, job.RunID)
	}
	runIDs = uniquePositiveIDs(runIDs)
	runs, err := findRunsByIDs(ctx, runIDs)
	if err != nil {
		return nil, err
	}
	runByID := make(map[int64]*actions_model.ActionRun, len(runs))
	for _, run := range runs {
		runByID[run.ID] = run
	}
	if len(runs) > 0 {
		if err := actions_model.RunList(runs).LoadRepos(ctx); err != nil {
			return nil, fmt.Errorf("load action repositories: %w", err)
		}
	}

	items := make([]*DashboardWorkerItem, 0, len(jobs))
	for _, job := range jobs {
		run := runByID[job.RunID]
		var repo *repo_model.Repository
		if run != nil {
			repo = run.Repo
		}
		task := taskByJobID[job.ID]
		var runner *actions_model.ActionRunner
		if task != nil {
			runner = runnerByID[task.RunnerID]
		}
		item := &DashboardWorkerItem{
			Runner:    runner,
			Job:       job,
			Run:       run,
			Repo:      repo,
			Status:    job.Status,
			IsRunning: job.Status == actions_model.StatusRunning,
			IsQueued:  job.Status == actions_model.StatusWaiting || job.Status == actions_model.StatusBlocked,
			Updated:   job.Updated,
		}
		item.Search = workerSearchText(item)
		items = append(items, item)
	}
	sortDashboardWorkerItems(items)
	stats.Items = items

	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	stats.SuccessLastDay, err = countRunsSince(ctx, actions_model.StatusSuccess, cutoff)
	if err != nil {
		return nil, err
	}
	stats.FailureLastDay, err = countRunsSince(ctx, actions_model.StatusFailure, cutoff)
	if err != nil {
		return nil, err
	}
	var last actions_model.ActionRun
	has, err := db.GetEngine(ctx).
		In("status", []actions_model.Status{actions_model.StatusSuccess, actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusSkipped}).
		Desc("stopped").
		Limit(1).
		Get(&last)
	if err != nil {
		return nil, fmt.Errorf("find last completed action run: %w", err)
	}
	if has {
		if err := last.LoadRepo(ctx); err != nil {
			return nil, fmt.Errorf("load last action repository: %w", err)
		}
		stats.LastCompleted = &last
	}
	return stats, nil
}

func populateRunnerStats(ctx context.Context, stats *DashboardStats, now time.Time) error {
	onlineCutoff := now.Add(-actions_model.RunnerOfflineTime).Unix()
	idleCutoff := now.Add(-actions_model.RunnerIdleTime).Unix()
	var err error
	if stats.TotalRunners, err = db.GetEngine(ctx).Count(new(actions_model.ActionRunner)); err != nil {
		return fmt.Errorf("count action runners: %w", err)
	}
	if stats.DisabledRunners, err = db.GetEngine(ctx).Where("is_disabled = ?", true).Count(new(actions_model.ActionRunner)); err != nil {
		return fmt.Errorf("count disabled action runners: %w", err)
	}
	if stats.ActiveRunners, err = db.GetEngine(ctx).
		Where("is_disabled = ?", false).
		And("last_online > ?", onlineCutoff).
		And("last_active > ?", idleCutoff).
		Count(new(actions_model.ActionRunner)); err != nil {
		return fmt.Errorf("count active action runners: %w", err)
	}
	if stats.IdleRunners, err = db.GetEngine(ctx).
		Where("is_disabled = ?", false).
		And("last_online > ?", onlineCutoff).
		And("last_active <= ?", idleCutoff).
		Count(new(actions_model.ActionRunner)); err != nil {
		return fmt.Errorf("count idle action runners: %w", err)
	}
	if stats.OfflineRunners, err = db.GetEngine(ctx).
		Where("is_disabled = ?", false).
		And("last_online <= ?", onlineCutoff).
		Count(new(actions_model.ActionRunner)); err != nil {
		return fmt.Errorf("count offline action runners: %w", err)
	}
	return nil
}

func findDashboardJobs(ctx context.Context, jobs *[]*actions_model.ActionRunJob, statuses ...actions_model.Status) error {
	if err := db.GetEngine(ctx).
		In("status", statuses).
		Desc("updated").
		Limit(dashboardWorkerLimit).
		Find(jobs); err != nil {
		return fmt.Errorf("find %s action jobs: %w", strings.Join(statusNames(statuses), ", "), err)
	}
	return nil
}

func countDashboardJobs(ctx context.Context, statuses ...actions_model.Status) (int64, error) {
	count, err := db.GetEngine(ctx).In("status", statuses).Count(new(actions_model.ActionRunJob))
	if err != nil {
		return 0, fmt.Errorf("count %s action jobs: %w", strings.Join(statusNames(statuses), ", "), err)
	}
	return count, nil
}

func statusNames(statuses []actions_model.Status) []string {
	names := make([]string, len(statuses))
	for i, status := range statuses {
		names[i] = status.String()
	}
	return names
}

func sortDashboardWorkerItems(items []*DashboardWorkerItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsRunning != items[j].IsRunning {
			return items[i].IsRunning
		}
		return items[i].Updated > items[j].Updated
	})
}

func findRunsByIDs(ctx context.Context, ids []int64) ([]*actions_model.ActionRun, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var runs []*actions_model.ActionRun
	if err := db.GetEngine(ctx).In("id", ids).Find(&runs); err != nil {
		return nil, fmt.Errorf("find active action runs: %w", err)
	}
	return runs, nil
}

func countRunsSince(ctx context.Context, status actions_model.Status, cutoff int64) (int64, error) {
	count, err := db.GetEngine(ctx).
		Where("status = ?", status).
		And("stopped >= ?", cutoff).
		Count(new(actions_model.ActionRun))
	if err != nil {
		return 0, fmt.Errorf("count %s action runs: %w", status, err)
	}
	return count, nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func workerSearchText(item *DashboardWorkerItem) string {
	if item == nil {
		return ""
	}
	parts := []string{item.Status.String()}
	if item.Job != nil {
		parts = append(parts, item.Job.Name, strconv.FormatInt(item.Job.ID, 10))
	}
	if item.Runner != nil {
		parts = append(parts, item.Runner.Name, strings.Join(item.Runner.AgentLabels, " "))
	}
	if item.Repo != nil {
		parts = append(parts, item.Repo.FullName())
	}
	if item.Run != nil {
		parts = append(parts, item.Run.CommitSHA, item.Run.WorkflowID)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
