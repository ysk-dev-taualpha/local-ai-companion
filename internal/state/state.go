package state

import (
	"fmt"
	"sync"
)

// State is the conversation state of the AI companion.
type State int

const (
	IDLE      State = iota // 待機中
	LISTENING              // ユーザー音声受信中
	THINKING               // 応答生成中
	SPEAKING               // 音声出力中
)

// String returns the Japanese label for the state.
func (s State) String() string {
	switch s {
	case IDLE:
		return "待機中"
	case LISTENING:
		return "受信中"
	case THINKING:
		return "思考中"
	case SPEAKING:
		return "発話中"
	default:
		return fmt.Sprintf("不明(%d)", s)
	}
}

// StateChangeCallback is called when the state machine transitions.
// from is the previous state, to is the new state.
type StateChangeCallback func(from, to State)

// StateMachine manages conversation state with validated transitions.
type StateMachine struct {
	mu          sync.RWMutex
	current     State
	onChange    StateChangeCallback
	transitions map[State]map[State]bool
}

// New creates a new StateMachine in IDLE state.
// If callback is non-nil, it is invoked on every valid state transition.
func New(callback StateChangeCallback) *StateMachine {
	sm := &StateMachine{
		current:  IDLE,
		onChange: callback,
		transitions: map[State]map[State]bool{
			IDLE:      {LISTENING: true},
			LISTENING: {THINKING: true, IDLE: true},
			THINKING:  {SPEAKING: true, IDLE: true},
			SPEAKING:  {IDLE: true},
		},
	}
	return sm
}

// Current returns the current state.
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// Transition attempts to change to the target state.
// Returns an error if the transition is not allowed.
func (sm *StateMachine) Transition(to State) error {
	sm.mu.Lock()
	if sm.current == to {
		sm.mu.Unlock()
		return nil
	}
	if !sm.transitions[sm.current][to] {
		from := sm.current
		sm.mu.Unlock()
		return fmt.Errorf("invalid transition: %s → %s", from, to)
	}
	from := sm.current
	sm.current = to
	onChange := sm.onChange
	sm.mu.Unlock()

	if onChange != nil {
		onChange(from, to)
	}
	return nil
}

// Reset forces the state machine back to IDLE regardless of current state.
// The callback is invoked if the state actually changes.
func (sm *StateMachine) Reset() {
	sm.mu.Lock()
	if sm.current != IDLE {
		from := sm.current
		sm.current = IDLE
		onChange := sm.onChange
		sm.mu.Unlock()

		if onChange != nil {
			onChange(from, IDLE)
		}
		return
	}
	sm.mu.Unlock()
}
