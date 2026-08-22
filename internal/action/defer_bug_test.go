package action

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"example.com/gflow/internal/model"
)

type trackedTransport struct {
	mu      sync.Mutex
	active  int
	maximum int
}

type trackedBody struct {
	io.Reader
	close func()
}

func (b *trackedBody) Close() error { b.close(); return nil }

func (t *trackedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.active++
	if t.active > t.maximum {
		t.maximum = t.active
	}
	t.mu.Unlock()
	body := &trackedBody{Reader: strings.NewReader("ok"), close: func() { t.mu.Lock(); t.active--; t.mu.Unlock() }}
	return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
}

func TestBug06_HTTPBatchClosesEachResponsePromptly(t *testing.T) {
	transport := &trackedTransport{}
	e := New()
	e.client = &http.Client{Transport: transport}
	n := model.Node{Config: map[string]any{"kind": "http_batch", "urls": []any{"http://one", "http://two", "http://three"}}}
	if _, err := e.Run(context.Background(), n, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if transport.maximum != 1 {
		t.Fatalf("responses kept open across batch: maximum=%d", transport.maximum)
	}
}
