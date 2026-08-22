package store

import (
	"example.com/gflow/internal/model"
	"sort"
	"testing"
	"time"
)

// TestListInstancesDoesNotMutateActiveNodeOrder guards against the cross-layer
// state-pollution bug where listing (and sorting the response copies of)
// CurrentNodeIDs reordered the live instance held in the store. The clone must
// produce a fully independent slice so that callers can sort their copies
// without disturbing the engine's activation order.
func TestListInstancesDoesNotMutateActiveNodeOrder(t *testing.T) {
	m := NewMemory()
	original := []string{"c", "a", "b"}
	i := &model.WorkflowInstance{
		ID:             "inst-1",
		WorkflowID:     "wf",
		BizKey:         "biz-1",
		Status:         model.Running,
		CurrentNodeIDs: append([]string(nil), original...),
		CreatedAt:      time.Now(),
	}
	if err := m.CreateInstance(i); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Single-instance lookup must not share the slice either.
	got, err := m.GetInstance("inst-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.CurrentNodeIDs[0], got.CurrentNodeIDs[1] = got.CurrentNodeIDs[1], got.CurrentNodeIDs[0]

	// ListInstances sorts its response copies; previously this sorted the live
	// slice in place because the clone shared the backing array.
	items := m.ListInstances(InstanceFilter{IncludeTerminal: true, Limit: 0})
	for _, item := range items {
		sort.Strings(item.CurrentNodeIDs)
	}

	live, err := m.GetInstance("inst-1")
	if err != nil {
		t.Fatalf("get live: %v", err)
	}
	for k, want := range original {
		if live.CurrentNodeIDs[k] != want {
			t.Fatalf("live order mutated: got %v, want %v", live.CurrentNodeIDs, original)
		}
	}

	// ListRunnable feeds the scheduler/engine Advance; it must hand over an
	// independent copy so engine mutation cannot reach the stored instance.
	_ = m.ListRunnable(time.Now())
}
