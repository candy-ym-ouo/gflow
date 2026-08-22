package store

import (
	"example.com/gflow/internal/model"
	"testing"
	"time"
)

// TestSubscribeCloseOnceDuringShutdown reproduces the reported ownership
// conflict: the store is closed while an SSE consumer is still blocked on its
// subscription channel. The consumer must observe a normal channel close
// (never a send-on-closed panic), and the subscriber's cancel func must be a
// no-op rather than a double close.
func TestSubscribeCloseOnceDuringShutdown(t *testing.T) {
	m := NewMemory()
	ch, cancel := m.Subscribe()

	// Simulate shutdown tearing down all subscriptions.
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Consumer sees end-of-stream, not an error.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed on shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("consumer blocked after store Close")
	}

	// The deferred cancel must be safe to invoke now; it must not double-close.
	cancel()
}

// TestSubscribeCancelAfterCloseDoesNotPanic ensures a late cancel (e.g. an
// SSE handler's defer firing after the server already tore the store down)
// never panics on close of a closed channel.
func TestSubscribeCancelAfterCloseDoesNotPanic(t *testing.T) {
	m := NewMemory()
	_, cancel := m.Subscribe()
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Should be a no-op, not a panic.
	cancel()
	cancel()
}

// TestSubscribeAfterCloseReceivesEndOfStream ensures clients connecting during
// shutdown get a clean end-of-stream channel rather than subscribing into a
// store that is being torn down.
func TestSubscribeAfterCloseReceivesEndOfStream(t *testing.T) {
	m := NewMemory()
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	ch, cancel := m.Subscribe()
	defer cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected already-closed channel")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after Subscribe on closed store")
	}
}

// TestSubscribeCancelRemovesSubscription verifies a normal, pre-close
// unsubscribe does not leave a dangling (blocking) subscription behind.
func TestSubscribeCancelRemovesSubscription(t *testing.T) {
	m := NewMemory()
	_, cancel := m.Subscribe()
	cancel()

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.subs) != 0 {
		t.Fatalf("expected zero subs after cancel, got %d", len(m.subs))
	}
}

// TestAppendEventReachesSubscriber verifies the fan-out still delivers events
// to a live subscription and that AppendEvent never panics when a subscriber
// has already canceled.
func TestAppendEventReachesSubscriber(t *testing.T) {
	m := NewMemory()
	ch, cancel := m.Subscribe()
	defer cancel()

	m.AppendEvent(model.NewEvent("inst", "", model.EventStarted, "tester", nil))

	select {
	case e, ok := <-ch:
		if !ok || e.Type != model.EventStarted {
			t.Fatalf("expected EventStarted, got %+v ok=%v", e, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}

	// Appending after a cancel must not panic on send to a (already removed)
	// subscription.
	cancel()
	m.AppendEvent(model.NewEvent("inst", "", model.EventNodeCompleted, "tester", nil))
}
