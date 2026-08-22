package store

import (
	"testing"

	"example.com/gflow/internal/model"
)

func TestBug05_GetInstanceReturnsIndependentCurrentNodes(t *testing.T) {
	s := NewMemory()
	i := &model.WorkflowInstance{ID: "instance", Version: 1, CurrentNodeIDs: []string{"review", "notify"}}
	if err := s.CreateInstance(i); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetInstance(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.CurrentNodeIDs[0] = "changed"
	stored, err := s.GetInstance(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CurrentNodeIDs[0] != "review" {
		t.Fatalf("stored nodes were mutated: %v", stored.CurrentNodeIDs)
	}
}
