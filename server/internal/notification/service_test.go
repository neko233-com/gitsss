// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
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
	processed, err := service.DispatchDue(context.Background(), "dispatcher", "request-2", 1)
	if err != nil {
		t.Fatal(err)
	}
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
		processed, err := service.DispatchDue(context.Background(), "dispatcher", "request", 1)
		if err != nil {
			t.Fatal(err)
		}
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

func TestPersistentServiceEncryptsAndRecoversInterruptedDelivery(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "gitsss.db")
	keyPath := filepath.Join(root, "gitsss.key")
	store, err := state.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := state.OpenVault(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &testAdapter{}
	service, err := NewPersistentService(testFactory{adapter: adapter}, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	configuration := Configuration{Channel: ChannelFeishu, WebhookURL: "https://open.feishu.cn/private", Secret: "super-secret"}
	if _, err := service.Configure("admin", "configure", configuration); err != nil {
		t.Fatal(err)
	}
	message, err := service.Enqueue(ChannelFeishu, "build-42", "构建失败", "请检查私密日志")
	if err != nil {
		t.Fatal(err)
	}
	_, claimed, _, ok, err := service.claimDue()
	if err != nil || !ok || claimed.Status != StatusDelivering {
		t.Fatalf("claimDue = %#v, %v, %v", claimed, ok, err)
	}
	configurationDocuments, err := store.List(configurationNamespace)
	if err != nil {
		t.Fatal(err)
	}
	messageDocuments, err := store.List(messageNamespace)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(configurationDocuments[0].Payload) + string(messageDocuments[0].Payload)
	if strings.Contains(stored, "super-secret") || strings.Contains(stored, "私密日志") || strings.Contains(stored, "open.feishu") {
		t.Fatalf("encrypted state leaked notification content: %s", stored)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = state.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service, err = NewPersistentService(testFactory{adapter: adapter}, &audit.Log{}, store, vault)
	if err != nil {
		t.Fatal(err)
	}
	recovered := service.Messages()
	if len(recovered) != 1 || recovered[0].Status != StatusRetrying || !strings.Contains(recovered[0].LastError, "outcome unknown") {
		t.Fatalf("recovered messages = %#v", recovered)
	}
	processed, err := service.DispatchDue(context.Background(), "dispatcher", "dispatch", 1)
	if err != nil || len(processed) != 1 || processed[0].Status != StatusDelivered || processed[0].Attempts != 2 {
		t.Fatalf("DispatchDue = %#v, %v", processed, err)
	}
	duplicate, err := service.Enqueue(ChannelFeishu, "build-42", "重复", "重复")
	if err != nil || duplicate.ID != message.ID {
		t.Fatalf("deduplicated message = %#v, %v", duplicate, err)
	}
}

func TestServiceReclaimsExpiredDeliveryLease(t *testing.T) {
	service := NewService(testFactory{adapter: &testAdapter{}}, &audit.Log{})
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.Configure("admin", "configure", Configuration{Channel: ChannelFeishu}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enqueue(ChannelFeishu, "lease", "构建失败", "检查日志"); err != nil {
		t.Fatal(err)
	}
	_, first, _, ok, err := service.claimDue()
	if err != nil || !ok || first.Attempts != 1 || first.Status != StatusDelivering {
		t.Fatalf("first claim = %#v, %v, %v", first, ok, err)
	}
	now = now.Add(deliveryLease)
	_, second, _, ok, err := service.claimDue()
	if err != nil || !ok || second.Attempts != 2 || second.Status != StatusDelivering {
		t.Fatalf("reclaimed delivery = %#v, %v, %v", second, ok, err)
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
