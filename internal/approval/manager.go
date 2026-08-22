package approval

import (
	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
	"fmt"
	"time"
)

type Manager struct{ s store.Store }

func New(s store.Store) *Manager { return &Manager{s: s} }
func (m *Manager) Create(i *model.WorkflowInstance, n model.Node) (*model.ApprovalTask, error) {
	a := []string{}
	if x, ok := n.Config["assignee"].(string); ok {
		a = []string{x}
	}
	t := &model.ApprovalTask{ID: fmt.Sprintf("task-%s-%s", i.ID, n.ID), InstanceID: i.ID, NodeID: n.ID, Mode: fmt.Sprint(n.Config["mode"]), Status: "PENDING", Assignees: a, PassRatio: 1}
	if t.Mode == "" {
		t.Mode = "single"
	}
	return t, m.s.SaveTask(t)
}
func (m *Manager) Approve(id, actor string) error {
	t, e := m.s.GetTask(id)
	if e != nil {
		return e
	}
	if t.Status != "PENDING" {
		return fmt.Errorf("task already handled")
	}
	if !t.HasAssignee(actor) {
		return fmt.Errorf("actor is not an assignee")
	}
	t.PassCount++
	if t.Mode == "countersign" && float64(t.PassCount)/float64(len(t.Assignees)) < t.PassRatio {
		return m.s.SaveTask(t)
	}
	t.Status = "APPROVED"
	return m.s.SaveTask(t)
}
func (m *Manager) Reject(id string) error {
	t, e := m.s.GetTask(id)
	if e != nil {
		return e
	}
	if t.Status != "PENDING" {
		return fmt.Errorf("task already handled")
	}
	t.Status = "REJECTED"
	return m.s.SaveTask(t)
}
func Deadline(d string) *time.Time {
	if d == "" {
		return nil
	}
	x, _ := time.ParseDuration(d)
	t := time.Now().Add(x)
	return &t
}
