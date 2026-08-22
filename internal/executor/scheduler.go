package executor

import (
	"context"
	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/store"
	"time"
)

type Scheduler struct {
	s        store.Store
	e        *engine.Engine
	interval time.Duration
	stop     chan struct{}
}

func New(s store.Store, e *engine.Engine, d time.Duration) *Scheduler {
	return &Scheduler{s: s, e: e, interval: d, stop: make(chan struct{})}
}
func (x *Scheduler) Start(ctx context.Context) {
	go func() {
		defer close(x.stop)
		t := time.NewTicker(x.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				for _, i := range x.s.ListRunnable(time.Now()) {
					_ = x.e.Advance(ctx, i)
				}
			case <-ctx.Done():
				return
			case <-x.stop:
				return
			}
		}
	}()
}
func (x *Scheduler) Stop() { close(x.stop) }
