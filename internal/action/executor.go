package action

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/gflow/internal/model"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Executor struct {
	client *http.Client

	mu        sync.RWMutex // guards functions and seen
	functions map[string]Function
	seen      map[string]map[string]any

	inflightMu sync.Mutex       // guards inflight
	inflight   map[string]*call // in-progress idempotent executions, keyed by idempotencyKey

	executions int64 // accessed atomically
}

// call tracks a single in-progress idempotent execution so concurrent callers
// for the same key block on the first execution and reuse its full result
// instead of re-running the side effect.
type call struct {
	done chan struct{}

	output map[string]any
	err    error
}

type Function func(context.Context, map[string]any) (map[string]any, error)

func New() *Executor {
	e := &Executor{client: &http.Client{Timeout: 10 * time.Second}, functions: map[string]Function{}, seen: map[string]map[string]any{}, inflight: map[string]*call{}}
	e.Register("echo", func(_ context.Context, input map[string]any) (map[string]any, error) { return input, nil })
	e.Register("set", setFunction)
	return e
}
func (e *Executor) Register(name string, fn Function) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.functions[name] = fn
}
func (e *Executor) Run(ctx context.Context, n model.Node, input map[string]any) (map[string]any, error) {
	atomic.AddInt64(&e.executions, 1)
	k := fmt.Sprint(n.Config["kind"])
	key := fmt.Sprint(n.Config["idempotencyKey"])
	if key != "" {
		return e.runIdempotent(ctx, key, n, input, k)
	}
	return e.execute(ctx, n, input, k)
}

// runIdempotent guarantees that for a given idempotency key the underlying
// action executes at most once: the first caller runs the action and caches
// its result; concurrent callers for the same key block until it completes and
// then receive a clone of the cached result. Different keys run concurrently.
func (e *Executor) runIdempotent(ctx context.Context, key string, n model.Node, input map[string]any, k string) (map[string]any, error) {
	e.mu.RLock()
	if value, ok := e.seen[key]; ok {
		e.mu.RUnlock()
		return clone(value), nil
	}
	e.mu.RUnlock()

	e.inflightMu.Lock()
	if c, ok := e.inflight[key]; ok {
		e.inflightMu.Unlock()
		select {
		case <-c.done:
			return clone(c.output), c.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	c := &call{done: make(chan struct{})}
	e.inflight[key] = c
	e.inflightMu.Unlock()

	output, err := e.execute(ctx, n, input, k)

	if err == nil {
		e.mu.Lock()
		e.seen[key] = clone(output)
		e.mu.Unlock()
	}
	c.output = output
	c.err = err
	close(c.done)

	e.inflightMu.Lock()
	delete(e.inflight, key)
	e.inflightMu.Unlock()

	return output, err
}

func (e *Executor) execute(ctx context.Context, n model.Node, input map[string]any, k string) (map[string]any, error) {
	var output map[string]any
	var err error
	switch k {
	case "wait":
		d, parseErr := time.ParseDuration(fmt.Sprint(n.Config["duration"]))
		if parseErr != nil {
			return nil, parseErr
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		output = input
	case "notify":
		output = map[string]any{"notified": true, "channel": n.Config["channel"], "input": input}
	case "function":
		name := fmt.Sprint(n.Config["name"])
		e.mu.RLock()
		fn, ok := e.functions[name]
		e.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unknown function %s", name)
		}
		output, err = fn(ctx, input)
	case "http":
		output, err = e.runHTTP(ctx, n, input)
	case "fail":
		return nil, fmt.Errorf("action failed")
	default:
		return nil, fmt.Errorf("unsupported action kind %s", k)
	}
	if err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]any{}
	}
	return output, nil
}

func (e *Executor) runHTTP(ctx context.Context, n model.Node, input map[string]any) (map[string]any, error) {
	url := fmt.Sprint(n.Config["url"])
	if url == "" {
		return nil, fmt.Errorf("http action requires url")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(fmt.Sprint(n.Config["method"]))
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("http action returned %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	out := map[string]any{"status": response.StatusCode}
	if len(data) > 0 {
		var decoded any
		if json.Unmarshal(data, &decoded) == nil {
			out["body"] = decoded
		} else {
			out["body"] = string(data)
		}
	}
	return out, nil
}
func setFunction(_ context.Context, input map[string]any) (map[string]any, error) {
	return clone(input), nil
}
func clone(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}
