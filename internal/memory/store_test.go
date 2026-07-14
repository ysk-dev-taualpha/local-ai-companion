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

func TestSaveTurnAndLoadTurns(t *testing.T) {
	s, err := NewInMemoryStore(3)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	sid := "turn-session"
	// Save 2 complete turns
	if err := s.SaveTurn(sid, "hello", "hi there"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTurn(sid, "how are you?", "I'm good!"); err != nil {
		t.Fatal(err)
	}

	turns, err := s.LoadTurns(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].UserText != "hello" {
		t.Errorf("unexpected turn 0 user: %s", turns[0].UserText)
	}
	if turns[0].AssistantText != "hi there" {
		t.Errorf("unexpected turn 0 assistant: %s", turns[0].AssistantText)
	}
	if turns[1].UserText != "how are you?" {
		t.Errorf("unexpected turn 1 user: %s", turns[1].UserText)
	}
	if turns[1].AssistantText != "I'm good!" {
		t.Errorf("unexpected turn 1 assistant: %s", turns[1].AssistantText)
	}

	// Verify backward compatibility: LoadHistory still works
	history, err := s.LoadHistory(sid)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 messages (2 turns), got %d", len(history))
	}
	if history[0].Role != "user" || history[1].Role != "assistant" {
		t.Errorf("unexpected order: %s, %s", history[0].Role, history[1].Role)
	}
}

func TestSaveTurnMaxTurns(t *testing.T) {
	s, err := NewInMemoryStore(2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	sid := "turn-max"
	// Save 4 turns (maxTurns=2, so only last 2 should remain = 4 messages)
	for i := 0; i < 4; i++ {
		if err := s.SaveTurn(sid, "u", "a"); err != nil {
			t.Fatal(err)
		}
	}

	turns, _ := s.LoadTurns(sid)
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
}
