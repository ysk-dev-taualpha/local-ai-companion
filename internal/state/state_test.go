package state

import "testing"

func TestNewStartsIdle(t *testing.T) {
	sm := New(nil)
	if sm.Current() != IDLE {
		t.Errorf("expected IDLE, got %s", sm.Current())
	}
}

func TestValidTransitions(t *testing.T) {
	// 会話フローの全有効遷移をテスト
	tests := []struct {
		from State
		to   State
	}{
		{IDLE, LISTENING},
		{LISTENING, THINKING},
		{THINKING, SPEAKING},
		{SPEAKING, IDLE},
		// Cancel paths
		{LISTENING, IDLE},
		{THINKING, IDLE},
	}

	sm := New(nil)
	for _, tt := range tests {
		// Reset to starting state
		sm.Reset()
		// Navigate to the "from" state via valid path
		if tt.from != IDLE {
			switch tt.from {
			case LISTENING:
				if err := sm.Transition(LISTENING); err != nil {
					t.Fatalf("setup: transition to LISTENING failed: %v", err)
				}
			case THINKING:
				if err := sm.Transition(LISTENING); err != nil {
					t.Fatalf("setup: transition to LISTENING failed: %v", err)
				}
				if err := sm.Transition(THINKING); err != nil {
					t.Fatalf("setup: transition to THINKING failed: %v", err)
				}
			case SPEAKING:
				if err := sm.Transition(LISTENING); err != nil {
					t.Fatalf("setup: transition to LISTENING failed: %v", err)
				}
				if err := sm.Transition(THINKING); err != nil {
					t.Fatalf("setup: transition to THINKING failed: %v", err)
				}
				if err := sm.Transition(SPEAKING); err != nil {
					t.Fatalf("setup: transition to SPEAKING failed: %v", err)
				}
			}
		}

		err := sm.Transition(tt.to)
		if err != nil {
			t.Errorf("expected valid transition %s → %s, got error: %v", tt.from, tt.to, err)
		}
		if sm.Current() != tt.to {
			t.Errorf("after %s → %s, expected current=%s, got %s", tt.from, tt.to, tt.to, sm.Current())
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		from State
		to   State
	}{
		{IDLE, THINKING},
		{IDLE, SPEAKING},
		{LISTENING, SPEAKING},
		{THINKING, LISTENING},
		{SPEAKING, LISTENING},
		{SPEAKING, THINKING},
	}

	for _, tt := range tests {
		sm := New(nil)
		// Navigate to "from" state
		if tt.from != IDLE {
			// Must use valid transitions to reach target "from"
			reachState(t, sm, tt.from)
		}

		err := sm.Transition(tt.to)
		if err == nil {
			t.Errorf("expected error for invalid transition %s → %s, got nil", tt.from, tt.to)
		}
	}
}

func TestSameStateTransition(t *testing.T) {
	sm := New(nil)
	// Same state transition should be no-op, no error
	if err := sm.Transition(IDLE); err != nil {
		t.Errorf("expected no error for same-state transition IDLE → IDLE, got: %v", err)
	}
	if sm.Current() != IDLE {
		t.Errorf("expected IDLE after same-state transition, got %s", sm.Current())
	}
}

func TestCallbackFires(t *testing.T) {
	var called bool
	var capturedFrom, capturedTo State

	sm := New(func(from, to State) {
		called = true
		capturedFrom = from
		capturedTo = to
	})

	if err := sm.Transition(LISTENING); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("expected callback to be called on transition")
	}
	if capturedFrom != IDLE {
		t.Errorf("expected from=IDLE, got %s", capturedFrom)
	}
	if capturedTo != LISTENING {
		t.Errorf("expected to=LISTENING, got %s", capturedTo)
	}
}

func TestCallbackNotCalledOnSameState(t *testing.T) {
	var called bool
	sm := New(func(from, to State) {
		called = true
	})

	// Same state transition should not fire callback
	if err := sm.Transition(IDLE); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("callback should not fire on same-state transition")
	}
}

func TestCallbackNotCalledOnInvalidTransition(t *testing.T) {
	var called bool
	sm := New(func(from, to State) {
		called = true
	})

	// Invalid transition should not fire callback
	_ = sm.Transition(THINKING) // expected error
	if called {
		t.Error("callback should not fire on invalid transition")
	}
}

func TestReset(t *testing.T) {
	sm := New(nil)
	// Go to a non-IDLE state
	if err := sm.Transition(LISTENING); err != nil {
		t.Fatal(err)
	}
	sm.Reset()
	if sm.Current() != IDLE {
		t.Errorf("expected IDLE after reset, got %s", sm.Current())
	}
}

func TestResetFiresCallback(t *testing.T) {
	var called bool
	var capturedFrom, capturedTo State

	sm := New(func(from, to State) {
		called = true
		capturedFrom = from
		capturedTo = to
	})

	// Go to LISTENING then reset
	if err := sm.Transition(LISTENING); err != nil {
		t.Fatal(err)
	}
	called = false // reset the flag

	sm.Reset()
	if !called {
		t.Error("expected callback on reset from non-IDLE state")
	}
	if capturedFrom != LISTENING {
		t.Errorf("expected from=LISTENING, got %s", capturedFrom)
	}
	if capturedTo != IDLE {
		t.Errorf("expected to=IDLE, got %s", capturedTo)
	}
}

func TestResetFromIdleNoCallback(t *testing.T) {
	var called bool
	sm := New(func(from, to State) {
		called = true
	})

	// Reset from IDLE should not fire callback
	sm.Reset()
	if called {
		t.Error("callback should not fire when resetting from IDLE")
	}
}

func TestFullConversationFlow(t *testing.T) {
	// 一度の完全な会話フロー: IDLE→LISTENING→THINKING→SPEAKING→IDLE
	sm := New(nil)

	steps := []State{LISTENING, THINKING, SPEAKING, IDLE}
	expected := []State{LISTENING, THINKING, SPEAKING, IDLE}

	for i, step := range steps {
		if err := sm.Transition(step); err != nil {
			t.Fatalf("step %d: transition to %s failed: %v", i, step, err)
		}
		if sm.Current() != expected[i] {
			t.Errorf("step %d: expected %s, got %s", i, expected[i], sm.Current())
		}
	}
}

func TestCancelDuringListening(t *testing.T) {
	sm := New(nil)
	sm.Transition(LISTENING)
	if err := sm.Transition(IDLE); err != nil {
		t.Errorf("expected cancel (LISTENING→IDLE) to succeed, got: %v", err)
	}
	if sm.Current() != IDLE {
		t.Errorf("expected IDLE after cancel, got %s", sm.Current())
	}
}

func TestCancelDuringThinking(t *testing.T) {
	sm := New(nil)
	sm.Transition(LISTENING)
	sm.Transition(THINKING)
	if err := sm.Transition(IDLE); err != nil {
		t.Errorf("expected cancel (THINKING→IDLE) to succeed, got: %v", err)
	}
	if sm.Current() != IDLE {
		t.Errorf("expected IDLE after cancel, got %s", sm.Current())
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{IDLE, "待機中"},
		{LISTENING, "受信中"},
		{THINKING, "思考中"},
		{SPEAKING, "発話中"},
		{State(99), "不明(99)"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// reachState navigates the state machine to the target state using valid transitions.
func reachState(t *testing.T, sm *StateMachine, target State) {
	t.Helper()
	switch target {
	case IDLE:
		// Already IDLE
	case LISTENING:
		if err := sm.Transition(LISTENING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
	case THINKING:
		if err := sm.Transition(LISTENING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
		if err := sm.Transition(THINKING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
	case SPEAKING:
		if err := sm.Transition(LISTENING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
		if err := sm.Transition(THINKING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
		if err := sm.Transition(SPEAKING); err != nil {
			t.Fatalf("reachState: %v", err)
		}
	}
}

// TestConcurrentTransitions ensures StateMachine is safe for concurrent use.
// Run with: go test -race -count=1 ./internal/state/
func TestConcurrentTransitions(t *testing.T) {
	sm := New(nil)

	// Simulate multiple goroutines performing transitions and reads concurrently.
	// This mimics real-world usage where WebSocket handlers and AI service callbacks
	// access the StateMachine from different goroutines.
	done := make(chan bool)
	n := 20

	for i := 0; i < n; i++ {
		go func() {
			// Read current state
			_ = sm.Current()
			done <- true
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}

	// Concurrent transitions: interleave valid transitions, reads, and resets
	errs := make(chan error, n*3)
	for i := 0; i < n; i++ {
		go func() {
			// Ignore errors — invalid transitions are expected in concurrent access
			_ = sm.Transition(LISTENING)
			errs <- nil
		}()
		go func() {
			_ = sm.Current()
			errs <- nil
		}()
		go func() {
			sm.Reset()
			errs <- nil
		}()
	}
	for i := 0; i < n*3; i++ {
		<-errs
	}

	// Final state should be valid
	final := sm.Current()
	if final < IDLE || final > SPEAKING {
		t.Errorf("invalid final state after concurrent access: %d", final)
	}
}
