package engine

import (
	"context"
	"testing"

	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
)

func TestBug07_ApprovalTasksAreIndependentAcrossInstances(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := model.WorkflowDefinition{ID: "flow", Version: 1, Status: "published", Nodes: []model.Node{{ID: "start", Type: "start"}, {ID: "review", Type: "approval", Config: map[string]any{"assignee": "alice"}}, {ID: "end", Type: "end"}}, Edges: []model.Edge{{From: "start", To: "review"}, {From: "review", To: "end"}}}
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	first, err := e.Start(d, "first", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.Start(d, "second", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Advance(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := e.Advance(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if got := len(s.ListTasksForInstance(first.ID)); got != 1 {
		t.Fatalf("first instance tasks=%d", got)
	}
	if got := len(s.ListTasksForInstance(second.ID)); got != 1 {
		t.Fatalf("second instance tasks=%d", got)
	}
}
