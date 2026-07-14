// Copyright 2026 gitsss Authors. All rights reserved.

package agentjob

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

func TestQueueClaimsJobOnceAndCompletesIdempotently(t *testing.T) {
	now := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	log := &audit.Log{}
	queue := NewQueue(log)
	created, err := queue.Create("admin", "create-request", KindAssessment, json.RawMessage(`{"agent_id":"old-server"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	claimed := make(chan Job, 2)
	for _, agentID := range []string{"agent-a", "agent-b"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			job, ok, err := queue.Claim(agentID, "claim-request", now)
			if err != nil {
				t.Errorf("Claim: %v", err)
			}
			if ok {
				claimed <- job
			}
		}()
	}
	wait.Wait()
	close(claimed)
	jobs := make([]Job, 0, 1)
	for job := range claimed {
		jobs = append(jobs, job)
	}
	if len(jobs) != 1 || jobs[0].ID != created.ID {
		t.Fatalf("claimed jobs = %#v", jobs)
	}
	result := json.RawMessage(`{"id":"assessment-1"}`)
	completed, err := queue.Complete(jobs[0].ClaimedBy, "complete-request", created.ID, result, "", now.Add(time.Second))
	if err != nil || completed.Status != StatusSucceeded {
		t.Fatalf("Complete = %#v, %v", completed, err)
	}
	repeated, err := queue.Complete(jobs[0].ClaimedBy, "repeated-request", created.ID, result, "", now.Add(2*time.Second))
	if err != nil || repeated.Status != StatusSucceeded {
		t.Fatalf("idempotent Complete = %#v, %v", repeated, err)
	}
	if _, err := queue.Complete("other-agent", "wrong-request", created.ID, nil, "failed", now); !errors.Is(err, ErrWrongAgent) {
		t.Fatalf("wrong Agent error = %v", err)
	}
	if len(log.Entries()) != 3 {
		t.Fatalf("audit entries = %#v", log.Entries())
	}
}

func TestQueueReleasesExpiredLease(t *testing.T) {
	now := time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC)
	queue := NewQueue(nil)
	created, err := queue.Create("admin", "request", KindBackupVerify, json.RawMessage(`{"input_path":"backup"}`), now)
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := queue.Claim("offline-agent", "claim", now)
	if err != nil || !ok || first.ID != created.ID {
		t.Fatalf("first Claim = %#v, %v, %v", first, ok, err)
	}
	if _, ok, err := queue.Claim("second-agent", "claim", now.Add(time.Minute)); err != nil || ok {
		t.Fatalf("active lease was reclaimed: %v, %v", ok, err)
	}
	second, ok, err := queue.Claim("second-agent", "claim", now.Add(leaseDuration))
	if err != nil || !ok || second.ClaimedBy != "second-agent" {
		t.Fatalf("expired lease Claim = %#v, %v, %v", second, ok, err)
	}
}

func TestQueueRejectsInvalidPayloadAndCompletion(t *testing.T) {
	queue := NewQueue(nil)
	if _, err := queue.Create("admin", "request", Kind("shell.execute"), json.RawMessage(`{}`), time.Now()); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("unsafe kind error = %v", err)
	}
	job, err := queue.Create("admin", "request", KindAssessment, json.RawMessage(`{}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := queue.Claim("agent", "claim", time.Now())
	if err != nil || !ok || claimed.ID != job.ID {
		t.Fatal("job was not claimed")
	}
	if _, err := queue.Complete("agent", "complete", job.ID, json.RawMessage(`{}`), "also failed", time.Now()); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("ambiguous completion error = %v", err)
	}
}

func TestQueueAcceptsEncryptedBackupExport(t *testing.T) {
	queue := NewQueue(nil)
	job, err := queue.Create("admin", "request", KindBackupExport, json.RawMessage(`{"expected_assessment_id":"assessment-0123456789abcdef"}`), time.Now())
	if err != nil || job.Kind != KindBackupExport || job.Status != StatusQueued {
		t.Fatalf("backup export job = %#v, %v", job, err)
	}
}

func TestQueueAcceptsIsolatedBackupRestore(t *testing.T) {
	queue := NewQueue(nil)
	job, err := queue.Create("admin", "request", KindBackupRestore, json.RawMessage(`{"expected_container_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`), time.Now())
	if err != nil || job.Kind != KindBackupRestore || job.Status != StatusQueued {
		t.Fatalf("backup restore job = %#v, %v", job, err)
	}
}

func TestQueueAcceptsPostRestoreAudit(t *testing.T) {
	queue := NewQueue(nil)
	job, err := queue.Create("admin", "request", KindBackupAudit, json.RawMessage(`{"expected_assessment_id":"assessment-0123456789abcdef"}`), time.Now())
	if err != nil || job.Kind != KindBackupAudit || job.Status != StatusQueued {
		t.Fatalf("backup audit job = %#v, %v", job, err)
	}
}
