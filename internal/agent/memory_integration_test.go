package agent

import (
	"testing"

	"github.com/ysk-dev-taualpha/local-ai-companion/runtime/internal/memory"
)

func TestLoop_MemoryStoreIntegration(t *testing.T) {
	// Create an in-memory store
	store, err := memory.NewInMemoryStore(10)
	if err != nil {
		t.Fatalf("NewInMemoryStore: %v", err)
	}
	defer store.Close()

	// Seed some history
	store.SaveMessage("sess-1", "user", "hello")
	store.SaveMessage("sess-1", "assistant", "hi there!")

	// Create a Loop with the memory store
	loop := NewLoop(nil, nil, nil, Config{
		MaxToolLoops: 0, // 0→defaults to 5, but no client means it'll fail on Chat
		MemoryStore:  store,
	})

	// Verify memoryStore is wired
	if loop.memoryStore == nil {
		t.Fatal("memoryStore not wired from Config")
	}

	// Verify history is loaded via store (we can't fully exercise Run without a real client,
	// but we can verify the store reference is correctly assigned)
	history, err := store.LoadHistory("sess-1")
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "hello" {
		t.Errorf("unexpected history[0]: %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "hi there!" {
		t.Errorf("unexpected history[1]: %+v", history[1])
	}

	t.Logf("memory store integration: %d history messages loaded", len(history))
}
