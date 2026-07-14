// Copyright 2026 gitsss Authors. All rights reserved.

package importcatalog

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestCatalogExposesSanitizedImportedDirectoryAndWorkItems(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	putImportedRow(t, store, "user", "00000000000000000001", `{"source_table":"user","ordinal":1,"values":{"id":1,"name":"artist","full_name":"Lead Artist","type":0,"is_active":1,"passwd":"must-not-leak","created_unix":1784073600}}`)
	putImportedRow(t, store, "user", "00000000000000000002", `{"source_table":"user","ordinal":2,"values":{"id":2,"name":"studio","type":1,"is_active":1}}`)
	putImportedRow(t, store, "issue", "00000000000000000001", `{"source_table":"issue","ordinal":1,"values":{"id":9,"repo_id":3,"index":4,"name":"Lighting pass","content":"Review scene","poster_id":1,"is_pull":0,"is_closed":1}}`)
	if err := store.Put("import-work-items", "00000000000000000003-00000000000000000001", json.RawMessage(`{"source_table":"issue","ordinal":1,"values":{"id":9,"repo_id":3,"index":4,"name":"Lighting pass","content":"Review scene","poster_id":1,"is_pull":0,"is_closed":1}}`)); err != nil {
		t.Fatal(err)
	}
	putImportedRow(t, store, "access", "00000000000000000001", `{"source_table":"access","ordinal":1,"values":{"user_id":1,"repo_id":3,"mode":2}}`)
	if err := store.Put("import-repository-history", "00000000000000000003-00000000000000000001", json.RawMessage(`{"source_table":"comment","ordinal":1,"values":{"id":11,"_repository_id":3,"issue_id":9,"poster_id":1,"content":"Looks good"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("import-repository-history", "00000000000000000003-00000000000000000002", json.RawMessage(`{"source_table":"release","ordinal":1,"values":{"id":12,"_repository_id":3,"name":"First release","tag_name":"v1.0.0","note":"Ready"}}`)); err != nil {
		t.Fatal(err)
	}
	catalog, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := catalog.Directory("", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(directory.Items) != 1 || directory.Items[0].Name != "artist" || directory.Items[0].DisplayName != "Lead Artist" || directory.Items[0].Organization || directory.NextCursor == "" {
		t.Fatalf("directory = %#v", directory)
	}
	payload, err := json.Marshal(directory)
	if err != nil || strings.Contains(string(payload), "passwd") || strings.Contains(string(payload), "must-not-leak") {
		t.Fatalf("directory payload = %s, %v", payload, err)
	}
	organizations, err := catalog.Directory(directory.NextCursor, 10)
	if err != nil || len(organizations.Items) != 1 || !organizations.Items[0].Organization {
		t.Fatalf("organizations = %#v, %v", organizations, err)
	}
	items, err := catalog.WorkItems("3", "", 10)
	if err != nil || len(items.Items) != 1 || items.Items[0].Repository != "3" || items.Items[0].Title != "Lighting pass" || !items.Items[0].Closed {
		t.Fatalf("work items = %#v, %v", items, err)
	}
	history, err := catalog.RepositoryHistory("3", "", 10)
	if err != nil || len(history.Items) != 2 || history.Items[0].Kind != "comment" || history.Items[0].Body != "Looks good" || history.Items[1].Tag != "v1.0.0" {
		t.Fatalf("repository history = %#v, %v", history, err)
	}
	access, err := catalog.Access("", 10)
	if err != nil || len(access.Items) != 1 || access.Items[0].Mode != 2 || access.Items[0].User != "1" {
		t.Fatalf("access = %#v, %v", access, err)
	}
}

func putImportedRow(t *testing.T, store *state.Store, table, id, payload string) {
	t.Helper()
	if err := store.Put("source-import-"+table, id, json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
}
