package session

import "testing"

func TestManagerKillHidesClosingSession(t *testing.T) {
	m := NewManager()
	s := m.Create("session-1", TypeWebSocket, nil)
	s.MarkActive()

	if got := len(m.List()); got != 1 {
		t.Fatalf("expected 1 visible session, got %d", got)
	}
	if !m.Kill("session-1") {
		t.Fatal("expected kill to succeed")
	}
	if s.Status() != StateClosing {
		t.Fatalf("expected session state %q, got %q", StateClosing, s.Status())
	}
	if got := len(m.List()); got != 0 {
		t.Fatalf("expected killed session to be hidden from list, got %d entries", got)
	}

	Cleanup(m, s, CloseInfo{Cause: CloseCauseKill})

	if _, ok := m.Get("session-1"); ok {
		t.Fatal("expected session to be removed after cleanup")
	}
	if s.Status() != StateClosed {
		t.Fatalf("expected session state %q after cleanup, got %q", StateClosed, s.Status())
	}
}
