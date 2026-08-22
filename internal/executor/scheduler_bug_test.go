package executor

import (
	"context"
	"testing"
	"time"

	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/store"
)

func TestBug01_SchedulerStopAfterContextCancellation(t *testing.T) {
	s := store.NewMemory()
	x := New(s, engine.New(s), time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	x.Start(ctx)
	cancel()
	select {
	case <-x.stop:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
	x.Stop()
}
