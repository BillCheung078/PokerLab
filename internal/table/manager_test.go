package table

import (
	"testing"
	"time"

	"pokerlab/internal/sim"
)

func TestCreateGetAndDeleteTable(t *testing.T) {
	manager := NewManagerWithEngine(sim.NewEngineWithConfig(sim.EngineConfig{
		IntraHandDelay: 5 * time.Millisecond,
		HandPause:      5 * time.Millisecond,
	}))

	created, err := manager.Create("session-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty table ID")
	}
	if created.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want %q", created.SessionID, "session-1")
	}

	got, ok := manager.Get(created.ID)
	if !ok {
		t.Fatal("expected Get() to find the created table")
	}
	if got.ID != created.ID {
		t.Fatalf("Get().ID = %q, want %q", got.ID, created.ID)
	}

	runtime, ok := manager.Runtime(created.ID)
	if !ok {
		t.Fatal("expected Runtime() to find the created runtime")
	}
	waitForRuntime(t, time.Second, func() bool { return len(runtime.Snapshot().History) >= 2 })

	deleted, ok := manager.Delete(created.ID)
	if !ok {
		t.Fatal("expected Delete() to succeed")
	}
	if deleted.ID != created.ID {
		t.Fatalf("Delete().ID = %q, want %q", deleted.ID, created.ID)
	}
	if snapshot := runtime.Snapshot(); snapshot.Running {
		t.Fatal("expected runtime to be cancelled after Delete()")
	}

	if _, ok := manager.Get(created.ID); ok {
		t.Fatal("expected table to be removed from registry")
	}
	if _, ok := manager.Runtime(created.ID); ok {
		t.Fatal("expected runtime to be removed from registry")
	}
}

func waitForRuntime(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for runtime condition")
}
