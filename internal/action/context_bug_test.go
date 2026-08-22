package action

import (
	"context"
	"errors"
	"testing"

	"example.com/gflow/internal/model"
)

func TestBug02_ActionHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n := model.Node{Type: "action", Config: map[string]any{"kind": "wait", "duration": "100ms"}}
	_, err := New().Run(ctx, n, map[string]any{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
