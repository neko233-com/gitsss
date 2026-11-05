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

func TestMatchesRoleLabels(t *testing.T) {
	if !MatchesRoleLabels(false, nil, nil) {
		t.Fatal("all roles must include unlabeled users")
	}
	if !MatchesRoleLabels(true, []string{"客户端", "Ops"}, []string{"策划", "ops"}) {
		t.Fatal("any selected role should match")
	}
	if MatchesRoleLabels(true, []string{"客户端"}, []string{"后端"}) {
		t.Fatal("unselected role matched")
	}
	if MatchesRoleLabels(true, []string{"客户端"}, nil) {
		t.Fatal("unlabeled user matched")
	}
}

func TestAuthorAliases(t *testing.T) {
	aliases := map[string][]string{"tandeqing": {"zaixiao", "yideqing"}, "luohaojun": {"neko233*"}, "other": {"neko233-special"}}
	for name, want := range map[string]string{"ZAIXIAO": "tandeqing", "yideqing": "tandeqing", "neko233-dev": "luohaojun", "neko233-special": "other", "unknown": ""} {
		if got := MatchAuthorAlias(name, aliases); got != want {
			t.Fatalf("%s: got %s, want %s", name, got, want)
		}
	}
	pushes := []Push{{RepositoryID: 1, Total: 3, Commits: []Commit{{ID: "1", AuthorName: "zaixiao", AuthorEmail: "wrong@example.com"}, {ID: "2", AuthorName: "neko233-dev", AuthorEmail: "unknown@example.com"}, {ID: "3", AuthorName: "none", AuthorEmail: "registered@example.com"}}}}
	grouped, err := GroupCommits(pushes, map[string]string{"tandeqing": "person", "luohaojun": "person"}, func(email string) (string, error) {
		if email == "registered@example.com" {
			return "tandeqing", nil
		}
		return "", nil
	}, aliases)
	if err != nil || len(grouped["tandeqing"]) != 2 || len(grouped["luohaojun"]) != 1 {
		t.Fatalf("unexpected grouping: %v %v", grouped, err)
	}
	for _, bad := range []map[string][]string{{"a": {"same"}, "b": {"SAME"}}, {"a": {"abc*"}, "b": {"abcd*"}}, {"a": {"*"}}} {
		if (Config{Schedule: Schedule{Time: "21:00"}, AuthorAliases: bad}).Validate() == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
}
