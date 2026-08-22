package action

import (
	"context"
	"sync"
	"testing"

	"example.com/gflow/internal/model"
)

func TestBug10_ConcurrentIdempotentActionsAreRaceFree(t *testing.T) {
	e := New()
	n := model.Node{Config: map[string]any{"kind": "notify", "idempotencyKey": "shared"}}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = e.Run(context.Background(), n, map[string]any{"value": 1})
		}()
	}
	close(start)
	wg.Wait()
}
