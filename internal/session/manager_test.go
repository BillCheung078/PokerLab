package session

import (
	"net/http/httptest"
	"testing"
)

func TestGetOrCreateSetsAndReusesCookie(t *testing.T) {
	manager := NewManager()

	firstReq := httptest.NewRequest("GET", "/", nil)
	firstRec := httptest.NewRecorder()

	firstSession, err := manager.GetOrCreate(firstRec, firstReq)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if firstSession.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	cookies := firstRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}

	secondReq := httptest.NewRequest("GET", "/", nil)
	secondReq.AddCookie(cookies[0])
	secondRec := httptest.NewRecorder()

	secondSession, err := manager.GetOrCreate(secondRec, secondReq)
	if err != nil {
		t.Fatalf("GetOrCreate() reuse error = %v", err)
	}

	if secondSession.ID != firstSession.ID {
		t.Fatalf("session IDs differ: got %q want %q", secondSession.ID, firstSession.ID)
	}
	if len(secondRec.Result().Cookies()) != 0 {
		t.Fatal("expected no new cookie when reusing an existing session")
	}
}

func TestAddTableEnforcesMaxTables(t *testing.T) {
	manager := NewManager()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	sess, err := manager.GetOrCreate(rec, req)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	for i := 0; i < manager.MaxTables(); i++ {
		if err := manager.AddTable(sess.ID, "table-"+string(rune('a'+i))); err != nil {
			t.Fatalf("AddTable() unexpected error at %d: %v", i, err)
		}
	}

	if got := manager.Count(sess.ID); got != manager.MaxTables() {
		t.Fatalf("Count() = %d, want %d", got, manager.MaxTables())
	}

	if err := manager.AddTable(sess.ID, "table-overflow"); err != ErrMaxTablesReached {
		t.Fatalf("AddTable() overflow error = %v, want %v", err, ErrMaxTablesReached)
	}
}

func TestRemoveTableUpdatesOwnership(t *testing.T) {
	manager := NewManager()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	sess, err := manager.GetOrCreate(rec, req)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if err := manager.AddTable(sess.ID, "table-1"); err != nil {
		t.Fatalf("AddTable() error = %v", err)
	}

	if !manager.OwnsTable(sess.ID, "table-1") {
		t.Fatal("expected session to own the table")
	}

	if ok := manager.RemoveTable(sess.ID, "table-1"); !ok {
		t.Fatal("expected RemoveTable() to report success")
	}
	if manager.OwnsTable(sess.ID, "table-1") {
		t.Fatal("expected ownership to be removed")
	}
}
