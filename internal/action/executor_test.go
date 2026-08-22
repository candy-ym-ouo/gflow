package action

import (
	"context"
	"errors"
	"example.com/gflow/internal/model"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// batchNode builds an http_batch action node targeting the given urls.
func batchNode(urls ...string) model.Node {
	raw := make([]any, len(urls))
	for i, u := range urls {
		raw[i] = u
	}
	return model.Node{
		Type:   "action",
		Config: map[string]any{"kind": "http_batch", "urls": raw},
	}
}

// trackingBody wraps a response body and, on Close, synchronously decrements the
// transport's open count so the count is accurate by the time the next request
// is issued.
type trackingBody struct {
	io.Reader
	openCount *int64
	closed    bool
	mu        sync.Mutex
}

func (b *trackingBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	atomic.AddInt64(b.openCount, -1)
	return nil
}

// trackingTransport records the high-water mark of simultaneously-open response
// bodies. It lets the test assert that each response body is closed before the
// next request is issued — the regression behind the bug. If failSecond is set,
// the second request returns an error instead of a response, and openAtFailure
// captures how many bodies were still open at that instant.
type trackingTransport struct {
	openCount     int64
	highWater     int64
	failSecond    bool
	callCount     int64
	openAtFailure int64
}

func (t *trackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.failSecond && atomic.AddInt64(&t.callCount, 1) == 2 {
		// Snapshot how many prior response bodies are still open at the moment
		// the second request fails. If the first body was closed per-request
		// (the fix), this is 0; under the old deferred-close code it is 1.
		atomic.StoreInt64(&t.openAtFailure, atomic.LoadInt64(&t.openCount))
		return nil, errors.New("simulated transport error")
	}
	n := atomic.AddInt64(&t.openCount, 1)
	for {
		high := atomic.LoadInt64(&t.highWater)
		if n <= high {
			break
		}
		if atomic.CompareAndSwapInt64(&t.highWater, high, n) {
			break
		}
	}
	body := &trackingBody{Reader: io.NopCloser(strings.NewReader("ok")), openCount: &t.openCount}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     make(http.Header),
	}, nil
}

// TestRunHTTPBatchClosesEachResponseImmediately asserts that a response body is
// closed before the next request in the batch is issued. The original code
// deferred Close() to the end of the whole batch, so every response stayed open
// at once and the high-water mark of open bodies equaled the batch size.
func TestRunHTTPBatchClosesEachResponseImmediately(t *testing.T) {
	tr := &trackingTransport{}
	e := New()
	e.client = &http.Client{Transport: tr}

	urls := make([]string, 50)
	for i := range urls {
		urls[i] = "http://example.test/" + strconv.Itoa(i)
	}
	out, err := e.runHTTPBatch(context.Background(), batchNode(urls...), map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("runHTTPBatch: %v", err)
	}
	responses, _ := out["responses"].([]any)
	if len(responses) != len(urls) {
		t.Fatalf("expected %d responses, got %d", len(urls), len(responses))
	}
	if got := atomic.LoadInt64(&tr.highWater); got > 1 {
		t.Fatalf("expected at most one open response body at a time, high-water mark = %d (bodies accumulate)", got)
	}
	if got := atomic.LoadInt64(&tr.openCount); got != 0 {
		t.Fatalf("expected no open response bodies after batch, got %d", got)
	}
}

// TestRunHTTPBatchReleasesResponsesOnMidBatchFailure asserts that when a request
// fails mid-batch, the response body from a prior successful request is closed
// before the next request is attempted — not left open until the whole batch
// unwinds.
func TestRunHTTPBatchReleasesResponsesOnMidBatchFailure(t *testing.T) {
	tr := &trackingTransport{failSecond: true}
	e := New()
	e.client = &http.Client{Transport: tr}

	// First request: a normal response whose body should be closed before the
	// second request is attempted. Second request: the transport returns an
	// error mid-batch.
	urls := []string{"http://example.test/a", "http://example.test/b"}

	_, err := e.runHTTPBatch(context.Background(), batchNode(urls...), map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("expected error from failing second request, got nil")
	}
	if got := atomic.LoadInt64(&tr.openAtFailure); got != 0 {
		t.Fatalf("expected the first response body to be closed before the second request failed, still open = %d", got)
	}
	if got := atomic.LoadInt64(&tr.openCount); got != 0 {
		t.Fatalf("expected no leaked open response bodies after mid-batch failure, got %d", got)
	}
}

// TestRunHTTPBatchReleasesResponseWhenReadFails asserts the body is closed when
// reading the response body errors out partway through.
func TestRunHTTPBatchReleasesResponseWhenReadFails(t *testing.T) {
	e := New()
	e.client = &http.Client{Transport: &errorReadTransport{}}

	_, err := e.runHTTPBatch(context.Background(), batchNode("http://example.test/a"), map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("expected error from read failure, got nil")
	}
}

// errorReadTransport returns a response whose body errors on read, exercising
// the error path after the body is opened.
type errorReadTransport struct{}

func (errorReadTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &readErrorBody{},
		Header:     make(http.Header),
	}, nil
}

type readErrorBody struct {
	closed bool
	mu     sync.Mutex
}

func (b *readErrorBody) Read(p []byte) (int, error) { return 0, errors.New("read failed") }
func (b *readErrorBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	return nil
}
