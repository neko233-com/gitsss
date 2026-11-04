// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package homelinks

import "strings"

type PageEntry struct {
	IndexedLink
	Scope string
}

type Page struct {
	Entries  []PageEntry
	Shared   []IndexedLink
	Personal []IndexedLink
	Total    int
	Number   int
	Pages    int
	Query    string
}

// FindPage keeps the rendered result bounded, including when the query matches every link.
func FindPage(shared, personal []Link, query string, page int) Page {
	query = strings.TrimSpace(query)
	runes := []rune(query)
	if len(runes) > 120 {
		query = string(runes[:120])
	}
	maxPages := max(1, (len(shared)+len(personal)+PageSize-1)/PageSize)
	page = min(max(page, 1), maxPages)
	result := Page{Number: page, Query: query, Entries: make([]PageEntry, 0, PageSize)}
	needle := strings.ToLower(query)
	start := (page - 1) * PageSize
	appendScope := func(scope string, links []Link) {
		for index, link := range links {
			if needle != "" && !strings.Contains(strings.ToLower(link.Title), needle) && !strings.Contains(strings.ToLower(link.URL), needle) {
				continue
			}
			if result.Total >= start && len(result.Entries) < PageSize {
				entry := IndexedLink{Index: index, Link: link}
				result.Entries = append(result.Entries, PageEntry{IndexedLink: entry, Scope: scope})
				if scope == "shared" {
					result.Shared = append(result.Shared, entry)
				} else {
					result.Personal = append(result.Personal, entry)
				}
			}
			result.Total++
		}
	}
	appendScope("shared", shared)
	appendScope("personal", personal)
	result.Pages = max(1, (result.Total+PageSize-1)/PageSize)
	if page > result.Pages {
		return FindPage(shared, personal, query, result.Pages)
	}
	return result
}
