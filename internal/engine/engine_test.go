package engine

import (
	"context"
	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
	"testing"
)

func definition(types ...model.Node) model.WorkflowDefinition {
	edges := []model.Edge{}
	for i := 0; i+1 < len(types); i++ {
		edges = append(edges, model.Edge{From: types[i].ID, To: types[i+1].ID})
	}
	return model.WorkflowDefinition{ID: "test", Status: "published", Version: 1, Nodes: types, Edges: edges}
}

func TestAdvanceActionFlow(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := definition(model.Node{ID: "start", Type: "start"}, model.Node{ID: "notify", Type: "action", Config: map[string]any{"kind": "notify"}}, model.Node{ID: "end", Type: "end"})
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	i, err := e.Start(d, "biz-1", map[string]any{"name": "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Advance(context.Background(), i); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetInstance(i.ID)
	if got.Status != model.Completed {
		t.Fatalf("status = %s", got.Status)
	}
	if len(s.Events(i.ID)) < 3 {
		t.Fatalf("expected execution events")
	}
}

func TestApprovalWaitsUntilApproved(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := definition(model.Node{ID: "start", Type: "start"}, model.Node{ID: "review", Type: "approval", Config: map[string]any{"assignee": "alice"}}, model.Node{ID: "end", Type: "end"})
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	i, err := e.Start(d, "biz-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Advance(context.Background(), i); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetInstance(i.ID)
	if got.Status != model.WaitingApproval {
		t.Fatalf("status = %s", got.Status)
	}
	tasks := s.ListTasks("alice")
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d", len(tasks))
	}
	if err = e.Approve(tasks[0].ID, "bob"); err == nil {
		t.Fatal("non-assignee approval should fail")
	}
	if err = e.Approve(tasks[0].ID, "alice"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetInstance(i.ID)
	if err = e.Advance(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetInstance(i.ID)
	if got.Status != model.Completed {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestDraftCannotStart(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := definition(model.Node{ID: "start", Type: "start"}, model.Node{ID: "end", Type: "end"})
	d.Status = "draft"
	if _, err := e.Start(d, "biz-3", nil); err == nil {
		t.Fatal("draft workflow should not start")
	}
}
