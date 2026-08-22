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

// Two instances reaching the same approval node must each get their own
// independent, actionable approval task. Regression test for the case where
// the second instance's task collided on id, SaveTask returned ErrDuplicate,
// and the engine still wrote the instance into WAITING_APPROVAL with no task.
func TestApprovalConcurrentInstancesGetIndependentTasks(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := definition(model.Node{ID: "start", Type: "start"}, model.Node{ID: "review", Type: "approval", Config: map[string]any{"assignee": "alice"}}, model.Node{ID: "end", Type: "end"})
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	a, err := e.Start(d, "biz-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.Start(d, "biz-b", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Advance(context.Background(), a); err != nil {
		t.Fatalf("advance first instance: %v", err)
	}
	if err = e.Advance(context.Background(), b); err != nil {
		t.Fatalf("advance second instance: %v", err)
	}
	for _, i := range []*model.WorkflowInstance{a, b} {
		got, _ := s.GetInstance(i.ID)
		if got.Status != model.WaitingApproval {
			t.Fatalf("instance %s status = %s, want %s", i.ID, got.Status, model.WaitingApproval)
		}
		tasks := s.ListTasksForInstance(i.ID)
		if len(tasks) != 1 {
			t.Fatalf("instance %s has %d tasks, want 1", i.ID, len(tasks))
		}
		if tasks[0].ID == "" || tasks[0].InstanceID != i.ID {
			t.Fatalf("instance %s has mis-scoped task %+v", i.ID, tasks[0])
		}
	}
	all := s.ListTasks("alice")
	if len(all) != 2 {
		t.Fatalf("assignee task list = %d, want 2", len(all))
	}
	if all[0].ID == all[1].ID {
		t.Fatalf("two instances share the same task id %q", all[0].ID)
	}
	// Approving the first instance's task must not touch the second.
	if err = e.Approve(all[0].ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if err = e.Approve(all[1].ID, "alice"); err != nil {
		t.Fatal(err)
	}
	ra, _ := s.GetInstance(a.ID)
	if err = e.Advance(context.Background(), ra); err != nil {
		t.Fatal(err)
	}
	rb, _ := s.GetInstance(b.ID)
	if err = e.Advance(context.Background(), rb); err != nil {
		t.Fatal(err)
	}
	for _, i := range []*model.WorkflowInstance{a, b} {
		got, _ := s.GetInstance(i.ID)
		if got.Status != model.Completed {
			t.Fatalf("instance %s status = %s, want %s", i.ID, got.Status, model.Completed)
		}
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
