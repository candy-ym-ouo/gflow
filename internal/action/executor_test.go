package action

import (
	"context"
	"example.com/gflow/internal/model"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunConcurrentInstances verifies that different instances can advance in
// parallel without racing on the shared executor state, and that the idempotent
// key executes the underlying action at most once while reusing its result.
func TestRunConcurrentInstances(t *testing.T) {
	e := New()

	// Distinct idempotent keys must run concurrently without serialization,
	// so count how many are executing simultaneously.
	var inflight int64
	var maxInflight int64
	e.Register("slow", func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		n := atomic.AddInt64(&inflight, 1)
		if n > atomic.LoadInt64(&maxInflight) {
			atomic.StoreInt64(&maxInflight, n)
		}
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		atomic.AddInt64(&inflight, -1)
		return map[string]any{"ok": true}, nil
	})

	// Same idempotent key must execute the function at most once even under
	// concurrency, with all callers sharing the full result.
	var calls int64
	e.Register("once", func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		atomic.AddInt64(&calls, 1)
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return map[string]any{"ran": true}, nil
	})

	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)

	// Half use distinct keys (parallelism); half share one key (dedup + reuse).
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var node model.Node
			if i%2 == 0 {
				node = model.Node{Config: map[string]any{"kind": "function", "name": "slow", "idempotencyKey": "slow-" + strconv.Itoa(i)}}
			} else {
				node = model.Node{Config: map[string]any{"kind": "function", "name": "once", "idempotencyKey": "shared-once"}}
			}
			out, err := e.Run(ctx, node, map[string]any{})
			if err != nil {
				t.Errorf("Run error: %v", err)
				return
			}
			if out == nil {
				t.Errorf("Run returned nil output")
			}
		}()
	}
	wg.Wait()

	if max := atomic.LoadInt64(&maxInflight); max < 2 {
		t.Errorf("expected distinct idempotent keys to run concurrently, max concurrent = %d", max)
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("expected shared idempotent key to execute once, got %d", got)
	}
}
