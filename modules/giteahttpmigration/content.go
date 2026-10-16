// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package giteahttpmigration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Label is a repository label copied by name. Target IDs are assigned on create.
type Label struct {
	ID          int64  `json:"id,omitempty"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	Exclusive   bool   `json:"exclusive,omitempty"`
}

// Milestone is a repository milestone copied by title.
type Milestone struct {
	ID          int64      `json:"id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	State       string     `json:"state"`
	DueOn       *time.Time `json:"due_on,omitempty"`
}

// IssueComment is a human comment. Review threads and attachments are not copied.
type IssueComment struct {
	Body string `json:"body"`
	User *User  `json:"user,omitempty"`
}

// Issue is a non-pull issue. Numbers are recreated in order on an empty tracker.
type Issue struct {
	Number    int64          `json:"number"`
	Title     string         `json:"title"`
	Body      string         `json:"body,omitempty"`
	State     string         `json:"state"`
	User      *User          `json:"user,omitempty"`
	Labels    []Label        `json:"labels,omitempty"`
	Milestone *Milestone     `json:"milestone,omitempty"`
	Assignees []User         `json:"assignees,omitempty"`
	Comments  []IssueComment `json:"comments,omitempty"`
	// PullRequest is set when a listing mixes pull requests into the issue feed.
	PullRequest *struct{} `json:"pull_request,omitempty"`
}

// PullRequest is recreated against branches that the Git mirror already pushed.
// Merged requests are closed on the target; the merge commit itself is not replayed.
type PullRequest struct {
	Number    int64          `json:"number"`
	Title     string         `json:"title"`
	Body      string         `json:"body,omitempty"`
	State     string         `json:"state"`
	Merged    bool           `json:"merged,omitempty"`
	User      *User          `json:"user,omitempty"`
	Head      *PullRef       `json:"head"`
	Base      *PullRef       `json:"base"`
	Labels    []Label        `json:"labels,omitempty"`
	Milestone *Milestone     `json:"milestone,omitempty"`
	Assignees []User         `json:"assignees,omitempty"`
	Comments  []IssueComment `json:"comments,omitempty"`
	Reviews   []PullReview   `json:"reviews,omitempty"`
}

// PullRef is the head or base of a pull request.
type PullRef struct {
	Ref  string      `json:"ref"`
	SHA  string      `json:"sha,omitempty"`
	Repo *Repository `json:"repo,omitempty"`
}

// Release is release metadata. Attachment bytes are downloaded during apply.
type Release struct {
	ID         int64          `json:"id,omitempty"`
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name,omitempty"`
	Body       string         `json:"body,omitempty"`
	Draft      bool           `json:"draft,omitempty"`
	Prerelease bool           `json:"prerelease,omitempty"`
	Target     string         `json:"target_commitish,omitempty"`
	Assets     []ReleaseAsset `json:"assets,omitempty"`
}

type contentStats struct {
	labels, milestones, issues, pulls, comments, releases, remaps int
	assets, wikiPages, reviews, packagesCopied, packagesSkipped   int
	skipped                                                       bool
	notes                                                         []string
}

func (m *Migrator) discoverRepositoryContent(ctx context.Context, ref RepositoryRef, plan *RepositoryPlan) error {
	if err := m.source.getAll(ctx, repoPath(ref)+"/labels", &plan.Labels); err != nil {
		return fmt.Errorf("list source labels for %s: %w", ref, err)
	}
	if err := m.source.getAll(ctx, repoPath(ref)+"/milestones?state=all", &plan.Milestones); err != nil {
		return fmt.Errorf("list source milestones for %s: %w", ref, err)
	}
	var issues []Issue
	if plan.Repository.HasIssues == nil || *plan.Repository.HasIssues {
		if err := m.source.getAll(ctx, repoPath(ref)+"/issues?state=all&type=issues", &issues); err != nil {
			return fmt.Errorf("list source issues for %s: %w", ref, err)
		}
	}
	for i := range issues {
		if issues[i].PullRequest != nil {
			continue
		}
		if err := m.source.getAll(ctx, repoPath(ref)+"/issues/"+strconv.FormatInt(issues[i].Number, 10)+"/comments", &issues[i].Comments); err != nil {
			return fmt.Errorf("list source comments for %s#%d: %w", ref, issues[i].Number, err)
		}
		plan.Issues = append(plan.Issues, issues[i])
	}
	if plan.Repository.HasPullRequests == nil || *plan.Repository.HasPullRequests {
		if err := m.source.getAll(ctx, repoPath(ref)+"/pulls?state=all", &plan.Pulls); err != nil {
			return fmt.Errorf("list source pull requests for %s: %w", ref, err)
		}
	}
	for i := range plan.Pulls {
		if err := m.source.getAll(ctx, repoPath(ref)+"/issues/"+strconv.FormatInt(plan.Pulls[i].Number, 10)+"/comments", &plan.Pulls[i].Comments); err != nil {
			return fmt.Errorf("list source pull comments for %s#%d: %w", ref, plan.Pulls[i].Number, err)
		}
	}
	slices.SortFunc(plan.Labels, func(a, b Label) int { return cmp.Compare(a.Name, b.Name) })
	slices.SortFunc(plan.Milestones, func(a, b Milestone) int { return cmp.Compare(a.Title, b.Title) })
	slices.SortFunc(plan.Issues, func(a, b Issue) int { return cmp.Compare(a.Number, b.Number) })
	slices.SortFunc(plan.Pulls, func(a, b PullRequest) int { return cmp.Compare(a.Number, b.Number) })
	if plan.Repository.HasReleases == nil || *plan.Repository.HasReleases {
		if err := m.source.getAll(ctx, repoPath(ref)+"/releases", &plan.Releases); err != nil {
			return fmt.Errorf("list source releases for %s: %w", ref, err)
		}
	}
	slices.SortFunc(plan.Releases, func(a, b Release) int { return cmp.Compare(a.TagName, b.TagName) })
	return m.discoverExtras(ctx, ref, plan)
}

func (m *Migrator) applyRepositoryContent(ctx context.Context, ref RepositoryRef, plan RepositoryPlan, created bool) (contentStats, error) {
	stats := contentStats{}
	if len(plan.Labels)+len(plan.Milestones)+len(plan.Issues)+len(plan.Pulls)+len(plan.Releases)+len(plan.Wiki)+len(plan.Packages) == 0 {
		return stats, nil
	}
	if !created {
		stats.skipped = true
		stats.notes = append(stats.notes, ref.String()+": existing repository was not modified, so issues, pull requests, and releases were not copied")
		return stats, nil
	}
	labelIDs := map[string]int64{}
	for _, label := range plan.Labels {
		if strings.TrimSpace(label.Name) == "" {
			return stats, fmt.Errorf("source label in %s has no name", ref)
		}
		payload := map[string]any{
			"name": label.Name, "color": normalizeLabelColor(label.Color),
			"description": label.Description, "exclusive": label.Exclusive,
		}
		var createdLabel Label
		if err := m.target.post(ctx, repoPath(ref)+"/labels", payload, &createdLabel); err != nil {
			return stats, fmt.Errorf("create target label %s/%s: %w", ref, label.Name, err)
		}
		labelIDs[strings.ToLower(label.Name)] = createdLabel.ID
		stats.labels++
	}
	milestoneIDs := map[string]int64{}
	for _, milestone := range plan.Milestones {
		if strings.TrimSpace(milestone.Title) == "" {
			return stats, fmt.Errorf("source milestone in %s has no title", ref)
		}
		payload := map[string]any{
			"title": milestone.Title, "description": milestone.Description, "state": milestoneState(milestone.State),
		}
		if milestone.DueOn != nil && !milestone.DueOn.IsZero() {
			payload["due_on"] = milestone.DueOn
		}
		var createdMilestone Milestone
		if err := m.target.post(ctx, repoPath(ref)+"/milestones", payload, &createdMilestone); err != nil {
			return stats, fmt.Errorf("create target milestone %s/%s: %w", ref, milestone.Title, err)
		}
		milestoneIDs[strings.ToLower(milestone.Title)] = createdMilestone.ID
		stats.milestones++
	}

	items := make([]workItem, 0, len(plan.Issues)+len(plan.Pulls))
	for i := range plan.Issues {
		items = append(items, workItem{number: plan.Issues[i].Number, issue: &plan.Issues[i], comments: plan.Issues[i].Comments})
	}
	for i := range plan.Pulls {
		items = append(items, workItem{number: plan.Pulls[i].Number, pull: &plan.Pulls[i], comments: plan.Pulls[i].Comments})
	}
	slices.SortFunc(items, func(a, b workItem) int { return cmp.Compare(a.number, b.number) })
	for i := 1; i < len(items); i++ {
		if items[i].number == items[i-1].number {
			return stats, fmt.Errorf("source %s has two tracker items numbered %d", ref, items[i].number)
		}
	}
	notedRemap := false
	for _, item := range items {
		number, kind, reviewCount, err := m.createWorkItem(ctx, ref, item, labelIDs, milestoneIDs)
		if err != nil {
			return stats, err
		}
		stats.reviews += reviewCount
		if kind == "issue" {
			stats.issues++
		} else {
			stats.pulls++
		}
		if number != item.number {
			stats.remaps++
			if !notedRemap {
				stats.notes = append(stats.notes, ref.String()+": tracker numbers diverged from the source; comments follow the new numbers")
				notedRemap = true
			}
		}
		for _, comment := range item.comments {
			if strings.TrimSpace(comment.Body) == "" {
				continue
			}
			if err := m.createComment(ctx, ref, number, comment); err != nil {
				return stats, err
			}
			stats.comments++
		}
	}
	for _, release := range plan.Releases {
		if strings.TrimSpace(release.TagName) == "" {
			return stats, fmt.Errorf("source release in %s has no tag", ref)
		}
		payload := map[string]any{
			"tag_name": release.TagName, "name": release.Name, "body": release.Body,
			"draft": release.Draft, "prerelease": release.Prerelease,
		}
		if release.Target != "" {
			payload["target_commitish"] = release.Target
		}
		var created Release
		var output any
		if len(release.Assets) > 0 {
			output = &created
		}
		if err := m.target.post(ctx, repoPath(ref)+"/releases", payload, output); err != nil {
			return stats, fmt.Errorf("create target release %s/%s: %w", ref, release.TagName, err)
		}
		stats.releases++
		copied, err := m.copyReleaseAssets(ctx, ref, created.ID, release.Assets)
		stats.assets += copied
		if err != nil {
			return stats, err
		}
	}
	if err := m.applyExtras(ctx, ref, plan, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

type workItem struct {
	number   int64
	issue    *Issue
	pull     *PullRequest
	comments []IssueComment
}

func (m *Migrator) createWorkItem(ctx context.Context, ref RepositoryRef, item workItem, labelIDs, milestoneIDs map[string]int64) (int64, string, int, error) {
	if item.pull != nil {
		return m.createPull(ctx, ref, *item.pull, labelIDs, milestoneIDs)
	}
	number, kind, err := m.createIssue(ctx, ref, *item.issue, labelIDs, milestoneIDs)
	return number, kind, 0, err
}

func (m *Migrator) createIssue(ctx context.Context, ref RepositoryRef, issue Issue, labelIDs, milestoneIDs map[string]int64) (int64, string, error) {
	if strings.TrimSpace(issue.Title) == "" {
		return 0, "", fmt.Errorf("source issue %s#%d has no title", ref, issue.Number)
	}
	ids, err := mappedLabelIDs(issue.Labels, labelIDs)
	if err != nil {
		return 0, "", fmt.Errorf("source issue %s#%d: %w", ref, issue.Number, err)
	}
	payload := map[string]any{
		"title": issue.Title, "body": issue.Body, "closed": !strings.EqualFold(issue.State, "open") && issue.State != "",
	}
	if names := userNames(issue.Assignees); len(names) > 0 {
		payload["assignees"] = names
	}
	if len(ids) > 0 {
		payload["labels"] = ids
	}
	if id := mappedMilestoneID(issue.Milestone, milestoneIDs); id != 0 {
		payload["milestone"] = id
	}
	var created struct {
		Number int64 `json:"number"`
	}
	if err := m.postAs(ctx, repoPath(ref)+"/issues", loginOf(issue.User), payload, &created); err != nil {
		return 0, "", fmt.Errorf("create target issue %s#%d: %w", ref, issue.Number, err)
	}
	if created.Number == 0 {
		return 0, "", fmt.Errorf("create target issue %s#%d: response has no number", ref, issue.Number)
	}
	return created.Number, "issue", nil
}

func (m *Migrator) createPull(ctx context.Context, ref RepositoryRef, pull PullRequest, labelIDs, milestoneIDs map[string]int64) (int64, string, int, error) {
	if strings.TrimSpace(pull.Title) == "" {
		return 0, "", 0, fmt.Errorf("source pull %s#%d has no title", ref, pull.Number)
	}
	head, err := pullHead(ref, pull)
	if err != nil {
		return 0, "", 0, err
	}
	ids, err := mappedLabelIDs(pull.Labels, labelIDs)
	if err != nil {
		return 0, "", 0, fmt.Errorf("source pull %s#%d: %w", ref, pull.Number, err)
	}
	payload := map[string]any{
		"title": pull.Title, "body": pull.Body, "head": head, "base": pull.Base.Ref,
	}
	if names := userNames(pull.Assignees); len(names) > 0 {
		payload["assignees"] = names
	}
	if len(ids) > 0 {
		payload["labels"] = ids
	}
	if id := mappedMilestoneID(pull.Milestone, milestoneIDs); id != 0 {
		payload["milestone"] = id
	}
	var created struct {
		Number int64 `json:"number"`
	}
	if err := m.postAs(ctx, repoPath(ref)+"/pulls", loginOf(pull.User), payload, &created); err != nil {
		return 0, "", 0, fmt.Errorf("create target pull %s#%d: %w", ref, pull.Number, err)
	}
	if created.Number == 0 {
		return 0, "", 0, fmt.Errorf("create target pull %s#%d: response has no number", ref, pull.Number)
	}
	// Reviews are submitted before close. Gitea rejects a new review on a closed request.
	copied, err := m.applyPullReviews(ctx, ref, created.Number, pull.Reviews)
	if err != nil {
		return 0, "", copied, err
	}
	if pull.Merged || (pull.State != "" && !strings.EqualFold(pull.State, "open")) {
		endpoint := repoPath(ref) + "/pulls/" + strconv.FormatInt(created.Number, 10)
		if err := m.target.patch(ctx, endpoint, map[string]string{"state": "closed"}, http.StatusCreated); err != nil {
			return 0, "", copied, fmt.Errorf("close target pull %s#%d: %w", ref, pull.Number, err)
		}
	}
	return created.Number, "pull", copied, nil
}

func (m *Migrator) createComment(ctx context.Context, ref RepositoryRef, number int64, comment IssueComment) error {
	author := loginOf(comment.User)
	endpoint := repoPath(ref) + "/issues/" + strconv.FormatInt(number, 10) + "/comments"
	client := m.target
	if author != "" {
		client = m.target.asUser(author)
	}
	err := client.post(ctx, endpoint, map[string]string{"body": comment.Body}, nil)
	if err == nil {
		return nil
	}
	if author == "" || !sudoRejected(err) {
		return fmt.Errorf("create target comment on %s#%d: %w", ref, number, err)
	}
	// The original author cannot be impersonated. Keep the text and name the author.
	fallback := "> migrated from @" + author + "\n\n" + comment.Body
	if err := m.target.post(ctx, endpoint, map[string]string{"body": fallback}, nil); err != nil {
		return fmt.Errorf("create target comment on %s#%d: %w", ref, number, err)
	}
	return nil
}

func (m *Migrator) postAs(ctx context.Context, endpoint, user string, input, output any) error {
	client := m.target
	if user != "" {
		client = m.target.asUser(user)
	}
	err := client.post(ctx, endpoint, input, output)
	if user != "" && sudoRejected(err) {
		return m.target.post(ctx, endpoint, input, output)
	}
	return err
}

func sudoRejected(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusForbidden
}

func loginOf(user *User) string {
	if user == nil {
		return ""
	}
	return strings.TrimSpace(user.UserName)
}

func userNames(users []User) []string {
	names := make([]string, 0, len(users))
	seen := map[string]struct{}{}
	for _, user := range users {
		name := strings.TrimSpace(user.UserName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names
}

func mappedLabelIDs(labels []Label, ids map[string]int64) ([]int64, error) {
	out := make([]int64, 0, len(labels))
	for _, label := range labels {
		id, ok := ids[strings.ToLower(label.Name)]
		if !ok {
			return nil, fmt.Errorf("label %q was not created on the target", label.Name)
		}
		out = append(out, id)
	}
	return out, nil
}

func mappedMilestoneID(milestone *Milestone, ids map[string]int64) int64 {
	if milestone == nil {
		return 0
	}
	return ids[strings.ToLower(milestone.Title)]
}

func milestoneState(state string) string {
	if strings.EqualFold(state, "closed") {
		return "closed"
	}
	return "open"
}

func normalizeLabelColor(color string) string {
	color = strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(color) != 3 && len(color) != 6 {
		return "ededed"
	}
	for _, r := range color {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return "ededed"
		}
	}
	return color
}

func pullHead(ref RepositoryRef, pull PullRequest) (string, error) {
	if pull.Head == nil || pull.Head.Ref == "" || pull.Base == nil || pull.Base.Ref == "" {
		return "", fmt.Errorf("source pull %s#%d is missing head or base", ref, pull.Number)
	}
	if pull.Head.Repo == nil || pull.Head.Repo.FullName == "" || strings.EqualFold(pull.Head.Repo.FullName, ref.String()) {
		return pull.Head.Ref, nil
	}
	owner := ""
	if pull.Head.Repo.Owner != nil {
		owner = pull.Head.Repo.Owner.UserName
	}
	if owner == "" {
		name, _, _ := strings.Cut(pull.Head.Repo.FullName, "/")
		owner = name
	}
	if owner == "" {
		return "", fmt.Errorf("source pull %s#%d head repository has no owner", ref, pull.Number)
	}
	return owner + ":" + pull.Head.Ref, nil
}
