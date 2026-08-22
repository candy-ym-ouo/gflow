package store

import (
	"errors"
	"testing"

	"example.com/gflow/internal/model"
)

func TestBug03_SaveWorkflowPreservesConflictError(t *testing.T) {
	s := NewMemory()
	d := model.WorkflowDefinition{ID: "flow", Version: 1, Status: "published"}
	if err := s.SaveWorkflow(d); err != nil {
		t.Fatal(err)
	}
	err := s.SaveWorkflow(d)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict in error chain, got %v", err)
	}
}
