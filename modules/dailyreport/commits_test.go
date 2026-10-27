// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package dailyreport

import "testing"

func TestGroupCommits(t *testing.T) {
	person := Commit{ID: "a", AuthorEmail: "person@example.invalid", Message: "change"}
	service := Commit{ID: "b", AuthorEmail: "service@example.invalid", Message: "build"}
	resolve := func(email string) (string, error) {
		return map[string]string{"person@example.invalid": "person", "service@example.invalid": "service"}[email], nil
	}
	pushes := []Push{{RepositoryID: 1, Total: 2, Commits: []Commit{person, service}}, {RepositoryID: 1, Total: 1, Commits: []Commit{person}}, {RepositoryID: 2, Total: 1, Commits: []Commit{person}}}
	got, err := GroupCommits(pushes, map[string]string{"person": "person", "service": "service"}, resolve)
	if err != nil || len(got) != 1 || len(got["person"]) != 2 {
		t.Fatalf("unexpected grouped commits: %v, %v", got, err)
	}
	pushes[0].Total++
	if _, err := GroupCommits(pushes, nil, resolve); err == nil {
		t.Fatal("truncated push accepted")
	}
}
