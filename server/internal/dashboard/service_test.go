// Copyright 2026 gitsss Authors. All rights reserved.

package dashboard

import (
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

func TestServiceKeepsRecentEventsAndAuditsPreviewClear(t *testing.T) {
	log := &audit.Log{}
	service := NewService(2, []Event{{ID: "newest"}, {ID: "older"}, {ID: "oldest"}}, log)

	snapshot := service.Snapshot()
	if len(snapshot.Events) != 2 || snapshot.Events[0].ID != "newest" || snapshot.Events[1].ID != "older" {
		t.Fatalf("events = %#v, want newest-first capacity of 2", snapshot.Events)
	}

	service.ClearPreview("agent-1", "request-1", time.Now())
	if events := service.Snapshot().Events; len(events) != 0 {
		t.Fatalf("events = %#v, want empty preview", events)
	}
	if entries := log.Entries(); len(entries) != 1 || entries[0].Action != "events.preview.clear" {
		t.Fatalf("audit entries = %#v, want clear event", entries)
	}
}
