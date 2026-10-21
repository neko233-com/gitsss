// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package receiveflow

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func packet(data string) string { return fmt.Sprintf("%04x%s", len(data)+4, data) }

func header(old, next, ref string) string {
	return packet(old+" "+next+" "+ref+"\x00report-status\n") + "0000PACK\x00fixture-bytes"
}

func TestReceiveCommandsPreserveEveryByte(t *testing.T) {
	for _, width := range []int{40, 64} {
		old, next := strings.Repeat("1", width), strings.Repeat("2", width)
		input := header(old, next, "refs/heads/main")
		reader, commands := readCommands(strings.NewReader(input))
		data, err := io.ReadAll(reader)
		if err != nil || string(data) != input || len(commands) != 1 || commands[0].old != old || commands[0].next != next {
			t.Fatal("receive input was changed or its hash format was misread")
		}
	}
	for _, input := range []string{"", "0000", "ffffpartial", "not-hex", packet("push-cert\x00report-status\n") + "certificate-body", header("invalid", "invalid", "refs/heads/main")} {
		reader, _ := readCommands(strings.NewReader(input))
		data, err := io.ReadAll(reader)
		if err != nil || string(data) != input {
			t.Fatalf("native error/extension bytes changed for %q", input)
		}
	}
}

func TestReadyPredecessorOverlapsIndependentReferences(t *testing.T) {
	var registry Registry
	a, b, c := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	_, release := registry.Register("repo.git", strings.NewReader(header(a, b, "refs/heads/main")))
	defer release()
	waiting := make(chan struct{})
	go func() {
		registry.Wait(context.Background(), "repo.git", []string{b}, []string{c}, []string{"refs/heads/main"})
		close(waiting)
	}()
	select {
	case <-waiting:
		t.Fatal("known predecessor was not awaited")
	case <-time.After(10 * time.Millisecond):
	}
	independent := make(chan struct{})
	go func() {
		registry.Wait(context.Background(), "repo.git", []string{b}, []string{c}, []string{"refs/heads/other"})
		close(independent)
	}()
	select {
	case <-independent:
	case <-time.After(time.Second):
		t.Fatal("unrelated refs were serialized")
	}
	registry.Committed("other-repo.git", []string{b}, []string{"refs/heads/main"})
	select {
	case <-waiting:
		t.Fatal("another repository released this dependency")
	default:
	}
	registry.Committed("repo.git", []string{b}, []string{"refs/heads/main"})
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("committed predecessor did not release validation")
	}
}

func TestFailedParentCleanupCancellationAndDuplicateLeases(t *testing.T) {
	var registry Registry
	a, b, c := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	_, first := registry.Register("repo.git", strings.NewReader(header(a, b, "refs/heads/main")))
	_, second := registry.Register("repo.git", strings.NewReader(header(a, b, "refs/heads/main")))
	first()
	first()
	value, exists := registry.entries.Load(key{repository: "repo.git", ref: "refs/heads/main", oid: b})
	if !exists || value.(*pending).done.Load() {
		t.Fatal("one duplicate request retired another active transaction")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	registry.Wait(ctx, "repo.git", []string{b}, []string{c}, []string{"refs/heads/main"})
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("cancelled receive waited for the dependency deadline")
	}
	second()
	if _, exists := registry.entries.Load(key{repository: "repo.git", ref: "refs/heads/main", oid: b}); exists || !value.(*pending).done.Load() {
		t.Fatal("failed/closed native transactions leaked waiters")
	}
}

func TestConcurrentRegistrationAndCleanup(t *testing.T) {
	var registry Registry
	a, b := strings.Repeat("1", 40), strings.Repeat("2", 40)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 100 {
				_, done := registry.Register("repo.git", strings.NewReader(header(a, b, "refs/heads/main")))
				registry.Committed("repo.git", []string{b}, []string{"refs/heads/main"})
				done()
			}
		})
	}
	workers.Wait()
	registry.entries.Range(func(_, _ any) bool { t.Error("completed receive registration leaked"); return false })
}
