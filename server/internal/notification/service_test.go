// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

type testFactory struct {
	adapter Adapter
}

func (f testFactory) Build(Configuration) (Adapter, error) {
	return f.adapter, nil
}

type testAdapter struct {
	err error
}

func (a *testAdapter) Deliver(context.Context, Message) error {
	return a.err
}

func TestServiceDeduplicatesAndDeliversOutboxMessage(t *testing.T) {
	log := &audit.Log{}
	adapter := &testAdapter{}
	service := NewService(testFactory{adapter: adapter}, log)
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.Configure("admin", "request-1", Configuration{Channel: ChannelFeishu}); err != nil {
		t.Fatal(err)
	}
	first, err := service.Enqueue(ChannelFeishu, "build-42", "构建失败", "请检查日志")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Enqueue(ChannelFeishu, "build-42", "重复", "重复")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate message ID = %q, want %q", second.ID, first.ID)
	}
	processed := service.DispatchDue(context.Background(), "dispatcher", "request-2", 1)
	if len(processed) != 1 || processed[0].Status != StatusDelivered {
		t.Fatalf("processed = %#v, want one delivered message", processed)
	}
	if entries := log.Entries(); len(entries) != 2 {
		t.Fatalf("audit entries = %d, want configure and delivery", len(entries))
	}
}

func TestServiceRetriesThenMovesMessageToDeadLetter(t *testing.T) {
	adapter := &testAdapter{err: errors.New("secret\nnetwork detail")}
	service := NewService(testFactory{adapter: adapter}, &audit.Log{})
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	_, _ = service.Configure("admin", "request", Configuration{Channel: ChannelWeCom})
	_, _ = service.Enqueue(ChannelWeCom, "release-1", "发布", "失败")
	for attempt := 1; attempt <= 4; attempt++ {
		processed := service.DispatchDue(context.Background(), "dispatcher", "request", 1)
		if len(processed) != 1 {
			t.Fatalf("attempt %d processed = %#v", attempt, processed)
		}
		if attempt < 4 && processed[0].Status != StatusRetrying {
			t.Fatalf("attempt %d status = %s, want retrying", attempt, processed[0].Status)
		}
		now = now.Add(time.Duration(1<<(attempt-1)) * time.Minute)
	}
	message := service.Messages()[0]
	if message.Status != StatusDead || message.LastError != "secret network detail" {
		t.Fatalf("dead letter = %#v", message)
	}
}

func TestChannelsMasksEveryEmailRecipient(t *testing.T) {
	service := NewService(testFactory{adapter: &testAdapter{}}, &audit.Log{})
	_, err := service.Configure("admin", "request", Configuration{Channel: ChannelEmail, Recipients: []string{"one@example.com", "two@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	destination := service.Channels()[0].Destination
	if strings.Contains(destination, "one@") || strings.Contains(destination, "two@") {
		t.Fatalf("destination = %q, want every recipient masked", destination)
	}
}
