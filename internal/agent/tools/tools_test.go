package tools

import (
	"context"
	"testing"
)

func TestAudioControl_ValidActions(t *testing.T) {
	a := NewAudioControl()

	for _, action := range []string{"speak", "stop", "pause", "resume"} {
		args := []byte(`{"action":"` + action + `"}`)
		result, err := a.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error for action %q: %v", action, err)
		}
		if result == "" {
			t.Fatalf("expected non-empty result for action %q", action)
		}
	}
}

func TestAudioControl_InvalidAction(t *testing.T) {
	a := NewAudioControl()
	args := []byte(`{"action":"invalid"}`)
	_, err := a.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestAudioControl_MissingAction(t *testing.T) {
	a := NewAudioControl()
	args := []byte(`{}`)
	_, err := a.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestSetState(t *testing.T) {
	store := NewDefaultStateStore("IDLE")
	s := NewSetState(store, store)

	args := []byte(`{"state":"THINKING"}`)
	result, err := s.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if store.Current() != "THINKING" {
		t.Fatalf("expected THINKING, got %s", store.Current())
	}
}

func TestSetState_EmptyState(t *testing.T) {
	store := NewDefaultStateStore("IDLE")
	s := NewSetState(store, store)

	args := []byte(`{"state":""}`)
	_, err := s.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for empty state")
	}
}
