// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package activityreport summarizes commit timestamps for day, week, and month views.
package activityreport

import "time"

// Bucket is one reporting period.
type Bucket struct {
	Label   string `json:"label"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Commits int    `json:"commits"`
}

// Summary is the machine-readable activity report.
type Summary struct {
	Daily   []Bucket `json:"daily"`
	Weekly  []Bucket `json:"weekly"`
	Monthly []Bucket `json:"monthly"`
}

// SummarizeUnix counts commit timestamps. times are unix seconds.
// The newest period is the one that contains now.
func SummarizeUnix(times []int64, now time.Time) Summary {
	loc := now.Location()
	now = now.In(loc)
	return Summary{
		Daily: countBuckets(times, now, 14, func(n time.Time, i int) (time.Time, time.Time, string) {
			day := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -i)
			return day, day.AddDate(0, 0, 1), day.Format("2006-01-02")
		}),
		Weekly: countBuckets(times, now, 8, func(n time.Time, i int) (time.Time, time.Time, string) {
			day := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
			weekStart := day.AddDate(0, 0, -int(day.Weekday()))
			start := weekStart.AddDate(0, 0, -7*i)
			return start, start.AddDate(0, 0, 7), start.Format("2006-01-02")
		}),
		Monthly: countBuckets(times, now, 12, func(n time.Time, i int) (time.Time, time.Time, string) {
			start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -i, 0)
			return start, start.AddDate(0, 1, 0), start.Format("2006-01")
		}),
	}
}

func countBuckets(times []int64, now time.Time, count int, bound func(time.Time, int) (time.Time, time.Time, string)) []Bucket {
	out := make([]Bucket, 0, count)
	for i := range count {
		start, end, label := bound(now, i)
		bucket := Bucket{Label: label, Start: start.Format(time.RFC3339), End: end.Format(time.RFC3339)}
		for _, ts := range times {
			instant := time.Unix(ts, 0).In(now.Location())
			if !instant.Before(start) && instant.Before(end) {
				bucket.Commits++
			}
		}
		out = append(out, bucket)
	}
	return out
}
