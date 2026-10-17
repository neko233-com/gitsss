// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/modules/cache"
	"github.com/neko233-com/gitsss/modules/git"
	"github.com/neko233-com/gitsss/modules/git/gitcmd"
)

const contributionCommitLimit = 50000

var (
	ErrContributionBusy  = errors.New("contribution workers busy")
	contributionWorkers  = make(chan struct{}, 2)
	contributionInFlight sync.Map
)

type ContributionTotals struct {
	Commits     int   `json:"commits"`
	Additions   int64 `json:"additions"`
	Deletions   int64 `json:"deletions"`
	Files       int   `json:"files"`
	BinaryFiles int   `json:"binary_files"`
}

type ContributionDay struct {
	Date string `json:"date"`
	ContributionTotals
}

type ContributionAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	ContributionTotals
	Days map[string]*ContributionDay `json:"days"`
}

type ContributionReport struct {
	Revision  string `json:"revision"`
	From      string `json:"from"`
	To        string `json:"to"`
	Timezone  string `json:"timezone"`
	Truncated bool   `json:"truncated"`
	Limit     int    `json:"limit"`
	ContributionTotals
	Authors []*ContributionAuthor `json:"authors"`
}

func (t *ContributionTotals) add(v ContributionTotals) {
	t.Commits += v.Commits
	t.Additions += v.Additions
	t.Deletions += v.Deletions
	t.Files += v.Files
	t.BinaryFiles += v.BinaryFiles
}

// GetContributionReport uses one bounded Git stream, never checks out files or reads LFS blobs.
func GetContributionReport(ctx context.Context, store cache.StringCache, repo *git.Repository, revision string, from, to time.Time, zone *time.Location) (*ContributionReport, error) {
	commit, err := repo.GetCommit(revision)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("contribution-report-v1/%s/%s/%s/%s/%s", repo.Path, commit.ID, from.Format(time.RFC3339), to.Format(time.RFC3339), zone)
	var cached ContributionReport
	if store.IsExist(key) {
		if _, cacheErr := store.GetJSON(key, &cached); cacheErr == nil {
			return &cached, nil
		}
	}
	if _, busy := contributionInFlight.LoadOrStore(key, struct{}{}); busy {
		return nil, ErrContributionBusy
	}
	defer contributionInFlight.Delete(key)
	select {
	case contributionWorkers <- struct{}{}:
		defer func() { <-contributionWorkers }()
	default:
		return nil, ErrContributionBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := gitcmd.NewCommand("log", "--no-merges", "--no-renames", "--use-mailmap", "--numstat", "-z", "--format=%x00commit:%H%x00%aN%x00%aE%x00%ct%x00").
		AddOptionValues("--until", to.Add(-time.Second).Format(time.RFC3339)).
		AddOptionFormat("--max-count=%d", contributionCommitLimit+1).
		AddDynamicArguments(commit.ID.String()).
		WithDir(repo.Path)
	if git.DefaultFeatures().CheckVersionAtLeast("2.35") {
		cmd.AddOptionValues("--since-as-filter", from.Format(time.RFC3339))
	}
	reader, closeReader := cmd.MakeStdoutPipe()
	defer closeReader()
	var report *ContributionReport
	err = cmd.WithPipelineFunc(func(gitCtx gitcmd.Context) error {
		var parseErr error
		report, parseErr = parseContributionStream(reader, from, to, zone)
		return parseErr
	}).RunWithStderr(ctx)
	if err != nil {
		return nil, err
	}
	report.Revision = commit.ID.String()
	_ = store.PutJSON(key, report, 600)
	return report, nil
}

func parseContributionStream(input io.Reader, from, to time.Time, zone *time.Location) (*ContributionReport, error) {
	report := &ContributionReport{From: from.Format(time.DateOnly), To: to.AddDate(0, 0, -1).Format(time.DateOnly), Timezone: zone.String(), Limit: contributionCommitLimit, Authors: []*ContributionAuthor{}}
	authors := map[string]*ContributionAuthor{}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, 0); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	var author *ContributionAuthor
	var day *ContributionDay
	seen := 0
	for scanner.Scan() {
		value := strings.TrimPrefix(scanner.Text(), "\n")
		if strings.HasPrefix(value, "commit:") {
			day = nil
			seen++
			if seen > contributionCommitLimit {
				report.Truncated = true
			}
			if !scanner.Scan() {
				return nil, io.ErrUnexpectedEOF
			}
			name := scanner.Text()
			if !scanner.Scan() {
				return nil, io.ErrUnexpectedEOF
			}
			email := strings.ToLower(scanner.Text())
			if !scanner.Scan() {
				return nil, io.ErrUnexpectedEOF
			}
			stamp, err := strconv.ParseInt(scanner.Text(), 10, 64)
			if err != nil {
				return nil, err
			}
			date := time.Unix(stamp, 0).In(zone)
			if date.Before(from) || !date.Before(to) {
				continue
			}
			if seen > contributionCommitLimit {
				continue
			}
			if email == "" {
				email = name
			}
			identity := sha256.Sum256([]byte(email))
			id := hex.EncodeToString(identity[:16])
			author = authors[id]
			if author == nil {
				author = &ContributionAuthor{ID: id, Name: name, Days: map[string]*ContributionDay{}}
				authors[id] = author
			}
			dateKey := date.Format(time.DateOnly)
			day = author.Days[dateKey]
			if day == nil {
				day = &ContributionDay{Date: dateKey}
				author.Days[dateKey] = day
			}
			day.Commits++
			continue
		}
		if day == nil || value == "" {
			continue
		}
		fields := strings.SplitN(value, "\t", 3)
		if len(fields) != 3 {
			return nil, errors.New("invalid contribution numstat")
		}
		day.Files++
		if fields[0] == "-" && fields[1] == "-" {
			day.BinaryFiles++
			continue
		}
		additions, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || additions < 0 {
			return nil, errors.New("invalid additions")
		}
		deletions, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || deletions < 0 {
			return nil, errors.New("invalid deletions")
		}
		day.Additions += additions
		day.Deletions += deletions
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for _, author := range authors {
		for _, day := range author.Days {
			author.add(day.ContributionTotals)
		}
		report.add(author.ContributionTotals)
		report.Authors = append(report.Authors, author)
	}
	sort.Slice(report.Authors, func(i, j int) bool {
		a, b := report.Authors[i], report.Authors[j]
		if a.Commits != b.Commits {
			return a.Commits > b.Commits
		}
		return a.ID < b.ID
	})
	return report, nil
}
