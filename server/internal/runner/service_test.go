// Copyright 2026 gitsss Authors. All rights reserved.

package runner

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

func TestServiceReconcilesBundledRunnerLifecycle(t *testing.T) {
	service := NewService("1.0.0")
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	install, err := service.Reconcile(Report{ID: "agent-1", Platform: "windows/amd64"}, now)
	if err != nil || install.Action != ActionInstall || install.Runner.Status != StatusProvisioning {
		t.Fatalf("install = %#v, %v, want provisioning install", install, err)
	}

	ready, err := service.Reconcile(Report{ID: "agent-1", Platform: "windows/amd64", Version: "1.0.0"}, now)
	if err != nil || ready.Action != ActionVerify || ready.Runner.Status != StatusReady {
		t.Fatalf("ready = %#v, %v, want ready verification", ready, err)
	}

	service = NewService("1.1.0")
	service.runners["agent-1"] = Runner{ID: "agent-1", Platform: "windows/amd64", Version: "1.0.0", Status: StatusReady}
	update, err := service.Reconcile(Report{ID: "agent-1", Platform: "windows/amd64", Version: "1.0.0"}, now)
	if err != nil || update.Action != ActionUpdate || update.Runner.Version != "1.1.0" {
		t.Fatalf("update = %#v, %v, want target update", update, err)
	}
}

func TestPersistentServiceRepairsAndMarksOfflineAcrossRestart(t *testing.T) {
	root := t.TempDir()
	store, err := state.Open(filepath.Join(root, "gitsss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	log := &audit.Log{}
	service, err := NewPersistentService("1.0.0", log, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	healthy := true
	report := Report{ID: "agent-1", Platform: "linux/amd64", Version: "1.0.0", Registered: true, Healthy: &healthy, DiskFreeBytes: 20 << 30}
	if install, err := service.ReconcileAs("runner-token", "request-1", report, now); err != nil || install.Action != ActionInstall {
		t.Fatalf("install = %#v, %v", install, err)
	}
	if ready, err := service.ReconcileAs("runner-token", "request-2", report, now.Add(time.Second)); err != nil || ready.Action != ActionVerify || !ready.Runner.Healthy {
		t.Fatalf("ready = %#v, %v", ready, err)
	}
	reopened, err := NewPersistentService("1.0.0", log, store)
	if err != nil {
		t.Fatal(err)
	}
	if runners := reopened.RunnersAt(now.Add(time.Minute)); len(runners) != 1 || runners[0].Status != StatusReady || runners[0].DiskFreeBytes != 20<<30 {
		t.Fatalf("reloaded runners = %#v", runners)
	}
	unhealthy := false
	report.Healthy = &unhealthy
	report.Error = "health endpoint unavailable"
	repair, err := reopened.ReconcileAs("runner-token", "request-3", report, now.Add(time.Minute))
	if err != nil || repair.Action != ActionRepair || repair.Runner.Status != StatusDegraded {
		t.Fatalf("repair = %#v, %v", repair, err)
	}
	if runners := reopened.RunnersAt(now.Add(4 * time.Minute)); len(runners) != 1 || runners[0].Status != StatusOffline || runners[0].Healthy {
		t.Fatalf("offline runners = %#v", runners)
	}
	if entries := log.Entries(); len(entries) != 3 || entries[0].Actor != "runner-token" {
		t.Fatalf("runner audit = %#v", entries)
	}
}

func TestServiceRejectsUnknownPlatformAndInvalidHealth(t *testing.T) {
	service := NewService("1.0.0")
	if _, err := service.Reconcile(Report{ID: "agent-1", Platform: "plan9/amd64"}, time.Now()); err != ErrInvalidReport {
		t.Fatalf("unknown platform error = %v", err)
	}
	if _, err := service.Reconcile(Report{ID: "agent-1", Platform: "linux/amd64", Error: "bad\nerror"}, time.Now()); err != ErrInvalidReport {
		t.Fatalf("invalid error summary = %v", err)
	}
}

func TestServicePausesLowDiskRunner(t *testing.T) {
	service := NewService("1.0.0")
	now := time.Now().UTC()
	healthy := true
	report := Report{ID: "agent-1", Platform: "linux/amd64", Version: "1.0.0", Registered: true, Healthy: &healthy, DiskFreeBytes: 4 << 30}
	if _, err := service.Reconcile(report, now); err != nil {
		t.Fatal(err)
	}
	paused, err := service.Reconcile(report, now.Add(time.Second))
	if err != nil || paused.Action != ActionPause || paused.Runner.Status != StatusPaused || paused.Runner.Healthy {
		t.Fatalf("paused runner = %#v, %v", paused, err)
	}
}
