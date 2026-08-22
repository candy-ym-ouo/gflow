package engine

import (
	"testing"

	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
)

func TestBug04_StartInitializesNilContext(t *testing.T) {
	s := store.NewMemory()
	e := New(s)
	d := model.WorkflowDefinition{ID: "flow", Version: 1, Status: "published"}
	i, err := e.Start(d, "biz-nil", nil)
	if err != nil {
		t.Fatal(err)
	}
	if i.Context == nil || i.Context["workflowId"] != "flow" {
		t.Fatalf("context was not initialized: %#v", i.Context)
	}
}
