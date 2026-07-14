// Copyright 2026 gitsss Authors. All rights reserved.

package runner

import (
	"testing"
	"time"
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
