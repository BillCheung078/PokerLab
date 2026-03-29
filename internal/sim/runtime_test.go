package sim

import (
	"testing"
	"time"
)

func TestNewTableRuntimeInitializesState(t *testing.T) {
	createdAt := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	runtime := NewTableRuntime("tbl_123", "sess_abc", createdAt)

	snapshot := runtime.Snapshot()
	if snapshot.ID != "tbl_123" {
		t.Fatalf("Snapshot().ID = %q, want %q", snapshot.ID, "tbl_123")
	}
	if snapshot.SessionID != "sess_abc" {
		t.Fatalf("Snapshot().SessionID = %q, want %q", snapshot.SessionID, "sess_abc")
	}
	if !snapshot.Running {
		t.Fatal("expected runtime to start in running state")
	}
	if snapshot.State.TableID != "tbl_123" {
		t.Fatalf("Snapshot().State.TableID = %q, want %q", snapshot.State.TableID, "tbl_123")
	}
	if snapshot.State.Status != "runtime_initialized" {
		t.Fatalf("Snapshot().State.Status = %q, want %q", snapshot.State.Status, "runtime_initialized")
	}
}

func TestAppendEventUpdatesHistoryAndSubscriber(t *testing.T) {
	runtime := NewTableRuntime("tbl_123", "sess_abc", time.Now())

	subID, ch, err := runtime.Subscribe(1)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer runtime.Unsubscribe(subID)

	runtime.AppendEvent(PokerEvent{
		Type:    "game_started",
		Payload: map[string]any{"game_number": 1},
	})

	select {
	case event := <-ch:
		if event.Type != "game_started" {
			t.Fatalf("subscriber event type = %q, want %q", event.Type, "game_started")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber event")
	}

	snapshot := runtime.Snapshot()
	if len(snapshot.History) != 1 {
		t.Fatalf("history length = %d, want 1", len(snapshot.History))
	}
	if snapshot.TotalEvents != 1 {
		t.Fatalf("total events = %d, want 1", snapshot.TotalEvents)
	}
	if snapshot.State.Status != "runtime_ready" {
		t.Fatalf("state status = %q, want %q", snapshot.State.Status, "runtime_ready")
	}
	if snapshot.State.LastEventAt.IsZero() {
		t.Fatal("expected LastEventAt to be updated")
	}
}

func TestCancelStopsRuntimeAndRejectsSubscribers(t *testing.T) {
	runtime := NewTableRuntime("tbl_123", "sess_abc", time.Now())

	id, ch, err := runtime.Subscribe(1)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	runtime.Cancel()

	snapshot := runtime.Snapshot()
	if snapshot.Running {
		t.Fatal("expected runtime to stop after cancel")
	}
	if snapshot.State.Status != "runtime_stopped" {
		t.Fatalf("state status = %q, want %q", snapshot.State.Status, "runtime_stopped")
	}
	if snapshot.SubscriberCount != 0 {
		t.Fatalf("subscriber count = %d, want 0", snapshot.SubscriberCount)
	}

	runtime.Unsubscribe(id)

	if _, _, err := runtime.Subscribe(1); err != ErrRuntimeStopped {
		t.Fatalf("Subscribe() error = %v, want %v", err, ErrRuntimeStopped)
	}

	select {
	case <-ch:
	case <-time.After(20 * time.Millisecond):
	}
}
