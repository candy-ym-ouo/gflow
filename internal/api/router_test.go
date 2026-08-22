package api

import (
	"context"
	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestStreamHandlerEndsOnStoreClose drives the SSE handler through a real
// http.ResponseWriter (httptest.ResponseRecorder, which implements
// http.Flusher) and tears the store down while the handler is blocked on its
// select. This reproduces the reported ownership conflict without a live
// socket: the handler must treat the closed subscription channel as a normal
// end of stream and return, rather than panicking ("event subscription
// closed") or double-closing the channel in its deferred cancel.
func TestStreamHandlerEndsOnStoreClose(t *testing.T) {
	s := store.NewMemory()
	e := engine.New(s)
	r := New(s, e)

	// The handler loops until the subscription channel closes or the request
	// context is canceled. We bound it with a request context so a regression
	// fails fast instead of hanging the test.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		r.stream(rec, httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil).WithContext(ctx))
		// On a clean shutdown the handler writes a terminal "event: end" frame.
		if !strings.Contains(rec.Body.String(), "event: end") {
			t.Errorf("expected terminal 'event: end' frame, got body: %q", rec.Body.String())
		}
		close(done)
	}()

	// Let the handler actually block on its select.
	time.Sleep(50 * time.Millisecond)
	if err := s.Close(); err != nil {
		t.Fatalf("store Close: %v", err)
	}

	select {
	case <-done:
		// Handler returned cleanly on the closed channel — no panic, no
		// double-close.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s of store Close")
	}
}

// TestStreamHandlerExitsOnContextCancel covers the client-disconnect path: the
// request context canceling must let the handler return and run its deferred
// cancel, leaving no blocking subscription behind.
func TestStreamHandlerExitsOnContextCancel(t *testing.T) {
	s := store.NewMemory()
	e := engine.New(s)
	r := New(s, e)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		r.stream(rec, httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil).WithContext(ctx))
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel() // simulate client disconnect

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return within 2s of context cancel")
	}

	// The handler's deferred cancel must have removed the subscription, so a
	// subsequent store Close drains immediately with nothing to reap.
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("store Close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("store Close blocked: a subscription was left behind")
	}
}
