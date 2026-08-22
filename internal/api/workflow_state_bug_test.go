package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/gflow/internal/engine"
	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
)

func TestBug08_WorkflowReadDoesNotMutateStoredActionConfig(t *testing.T) {
	s := store.NewMemory()
	d := model.WorkflowDefinition{ID: "flow", Nodes: []model.Node{{ID: "call", Type: "action", Config: map[string]any{"kind": "http", "url": "http://service"}}}}
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/flow", nil)
	res := httptest.NewRecorder()
	New(s, engine.New(s)).Routes().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d", res.Code)
	}
	stored, err := s.GetWorkflow("flow")
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.Nodes[0].Config["url"]; got != "http://service" {
		t.Fatalf("stored url was mutated: %v", got)
	}
}
