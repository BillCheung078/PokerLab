package sim

import (
	"testing"
	"time"
)

func TestEngineStartProducesIndependentRuntimeProgress(t *testing.T) {
	engine := NewEngineWithConfig(EngineConfig{
		IntraHandDelay: 5 * time.Millisecond,
		HandPause:      5 * time.Millisecond,
	})

	first := NewTableRuntime("tbl_one", "sess_1", time.Now())
	second := NewTableRuntime("tbl_two", "sess_2", time.Now())
	defer first.Cancel()
	defer second.Cancel()

	engine.Start(first)
	engine.Start(second)

	waitFor(t, time.Second, func() bool {
		return len(first.Snapshot().History) >= 3 && len(second.Snapshot().History) >= 3
	})

	firstSnapshot := first.Snapshot()
	secondSnapshot := second.Snapshot()

	if firstSnapshot.State.GameNumber == 0 || secondSnapshot.State.GameNumber == 0 {
		t.Fatal("expected both runtimes to advance into a hand")
	}
	if firstSnapshot.History[0].TableID != "tbl_one" {
		t.Fatalf("first runtime event table ID = %q, want %q", firstSnapshot.History[0].TableID, "tbl_one")
	}
	if secondSnapshot.History[0].TableID != "tbl_two" {
		t.Fatalf("second runtime event table ID = %q, want %q", secondSnapshot.History[0].TableID, "tbl_two")
	}
}

func TestEngineStopsGeneratingEventsAfterCancel(t *testing.T) {
	engine := NewEngineWithConfig(EngineConfig{
		IntraHandDelay: 5 * time.Millisecond,
		HandPause:      5 * time.Millisecond,
	})

	runtime := NewTableRuntime("tbl_stop", "sess_1", time.Now())
	engine.Start(runtime)

	waitFor(t, time.Second, func() bool {
		return len(runtime.Snapshot().History) >= 2
	})

	runtime.Cancel()
	waitFor(t, time.Second, func() bool {
		return !runtime.Snapshot().Running
	})

	historyLen := len(runtime.Snapshot().History)
	time.Sleep(30 * time.Millisecond)

	if got := len(runtime.Snapshot().History); got != historyLen {
		t.Fatalf("history length after cancel = %d, want %d", got, historyLen)
	}
}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for condition")
}
