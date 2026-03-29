package table

import "testing"

func TestCreateGetAndDeleteTable(t *testing.T) {
	manager := NewManager()

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

	deleted, ok := manager.Delete(created.ID)
	if !ok {
		t.Fatal("expected Delete() to succeed")
	}
	if deleted.ID != created.ID {
		t.Fatalf("Delete().ID = %q, want %q", deleted.ID, created.ID)
	}

	if _, ok := manager.Get(created.ID); ok {
		t.Fatal("expected table to be removed from registry")
	}
}
