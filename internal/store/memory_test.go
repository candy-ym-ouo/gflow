package store

import (
	"testing"

	"example.com/gflow/internal/model"
)

// TestGetWorkflowIsolatesNodeConfig guards against the shared-state regression
// where mutating a workflow returned by GetWorkflow (e.g. the API layer
// scrubbing "url" from an HTTP action node) leaked back into the stored
// definition and broke subsequent instances with "http action requires url".
func TestGetWorkflowIsolatesNodeConfig(t *testing.T) {
	m := NewMemory()
	d := model.WorkflowDefinition{
		ID:      "wf-http",
		Name:    "HTTP flow",
		Status:  "published",
		Version: 1,
		Nodes: []model.Node{
			{ID: "start", Type: "start"},
			{ID: "call", Type: "action", Config: map[string]any{"kind": "http", "url": "https://example.com"}},
			{ID: "end", Type: "end"},
		},
		Edges: []model.Edge{
			{From: "start", To: "call"},
			{From: "call", To: "end"},
		},
	}
	if err := m.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}

	// Simulate the API layer scrubbing "url" from the response definition.
	got, err := m.GetWorkflow("wf-http")
	if err != nil {
		t.Fatal(err)
	}
	for i := range got.Nodes {
		if got.Nodes[i].Type == "action" {
			delete(got.Nodes[i].Config, "url")
		}
	}

	again, err := m.GetWorkflow("wf-http")
	if err != nil {
		t.Fatal(err)
	}
	node := again.Node("call")
	if node == nil {
		t.Fatal("missing action node")
	}
	if got, ok := node.Config["url"].(string); !ok || got == "" {
		t.Fatalf("stored action url was scrubbed by a read: config=%v", node.Config)
	}
}

// TestSaveWorkflowIsolatesInput ensures the store does not retain maps shared
// with the caller's definition, so later caller-side mutation cannot corrupt
// persisted state.
func TestSaveWorkflowIsolatesInput(t *testing.T) {
	m := NewMemory()
	d := model.WorkflowDefinition{
		ID:      "wf",
		Status:  "draft",
		Version: 1,
		Nodes: []model.Node{
			{ID: "start", Type: "start"},
			{ID: "end", Type: "end"},
		},
	}
	if err := m.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	d.Version = 2
	if err := m.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetWorkflow("wf")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Fatalf("version = %d, want 2", got.Version)
	}
}
