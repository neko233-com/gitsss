// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/neko233-com/gitsss/modules/homelinks"
	"github.com/neko233-com/gitsss/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomeLinksPagination(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	state, err := homelinks.Load(t.Context(), 2)
	require.NoError(t, err)
	links := make([]homelinks.Link, homelinks.MaxLinks)
	for i := range links {
		links[i] = homelinks.Link{Title: fmt.Sprintf("Link %05d", i), URL: fmt.Sprintf("https://example.test/%d", i)}
	}
	require.NoError(t, homelinks.Save(t.Context(), 2, state.Revision(), links))
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/dashboard?tab=links&links_page=417"), http.StatusOK)
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, 16, doc.Find("[data-home-links-results] .home-link").Length())
	assert.Equal(t, 16, doc.Find("[data-home-links-form] [name=page_index]").Length())
	assert.Contains(t, doc.Find("[data-home-links-results]").Text(), "Link 09999")
	revision, exists := doc.Find("[data-home-links-form] [name=revision]").Attr("value")
	require.True(t, exists)
	values := url.Values{"scope": {"personal"}, "operation": {"page"}, "revision": {revision}, "return_path": {"/org/org3/dashboard"}, "links_page": {"417"}, "page_index": {"9999"}, "item_index": {"9999", "-1"}, "title": {"Last changed", ""}, "url": {"https://changed.test", ""}}
	session.MakeRequest(t, NewRequestWithURLValues(t, "POST", "/home-links", values), http.StatusSeeOther)
	saved, err := homelinks.Load(t.Context(), 2)
	require.NoError(t, err)
	assert.Equal(t, links[9998], saved.Links[9998])
	assert.Equal(t, "Last changed", saved.Links[9999].Title)
	values = url.Values{"scope": {"personal"}, "operation": {"page"}, "revision": {saved.Revision()}, "return_path": {"/org/org3/dashboard"}, "links_page": {"417"}, "page_index": {"9999"}, "item_index": {"9999", "-1"}, "delete_index": {"9999"}, "title": {"Last changed", "Replacement"}, "url": {"https://changed.test", "https://replacement.test"}}
	session.MakeRequest(t, NewRequestWithURLValues(t, "POST", "/home-links", values), http.StatusSeeOther)
	saved, err = homelinks.Load(t.Context(), 2)
	require.NoError(t, err)
	assert.Len(t, saved.Links, homelinks.MaxLinks)
	assert.Equal(t, "Replacement", saved.Links[9999].Title)
	resp = session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/dashboard?tab=links&links_query=replacement"), http.StatusOK)
	doc, err = goquery.NewDocumentFromReader(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, 1, doc.Find("[data-home-links-results] .home-link").Length())
}
