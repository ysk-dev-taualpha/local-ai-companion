package memory

import (
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	s, err := NewInMemoryStore(3)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	sid := "test-session"
	if err := s.SaveMessage(sid, "user", "こんにちは"); err != nil {
		t.Fatal(err)
	}
	s.SaveMessage(sid, "assistant", "こんにちは！")
	s.SaveMessage(sid, "user", "元気？")

	history, err := s.LoadHistory(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3, got %d", len(history))
	}
	if history[0].Content != "こんにちは" {
		t.Errorf("unexpected: %s", history[0].Content)
	}
}

func TestMaxTurns(t *testing.T) {
	s, err := NewInMemoryStore(2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	sid := "max-turns"
	for i := 0; i < 5; i++ {
		s.SaveMessage(sid, "user", "msg")
	}

	history, _ := s.LoadHistory(sid)
	if len(history) != 2 {
		t.Fatalf("expected 2, got %d", len(history))
	}
}

func TestCleanOldSessions(t *testing.T) {
	s, err := NewInMemoryStore(5)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage("old", "user", "test")
	if err := s.CleanOldSessions(0); err != nil {
		t.Fatal(err)
	}
	history, _ := s.LoadHistory("old")
	if len(history) != 0 {
		t.Errorf("expected empty, got %d", len(history))
	}
}

func TestCleanNewSession(t *testing.T) {
	s, err := NewInMemoryStore(5)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.SaveMessage("new", "user", "test")
	if err := s.CleanOldSessions(24 * time.Hour); err != nil {
		t.Fatal(err)
	}
	history, _ := s.LoadHistory("new")
	if len(history) != 1 {
		t.Errorf("expected 1, got %d", len(history))
	}
}
