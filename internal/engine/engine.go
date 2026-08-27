package engine

import (
	"context"
	"example.com/gflow/internal/action"
	"example.com/gflow/internal/approval"
	"example.com/gflow/internal/condition"
	"example.com/gflow/internal/model"
	"example.com/gflow/internal/store"
	"fmt"
	"time"
)

type Engine struct {
	s  store.Store
	a  *action.Executor
	ap *approval.Manager
}

func New(s store.Store) *Engine { return &Engine{s: s, a: action.New(), ap: approval.New(s)} }
func (e *Engine) Start(w model.WorkflowDefinition, biz string, ctx map[string]any) (*model.WorkflowInstance, error) {
	if w.Status != "published" {
		return nil, fmt.Errorf("workflow %s is not published", w.ID)
	}
	if biz == "" {
		return nil, fmt.Errorf("business key is required")
	}
	if existing, lookupErr := e.s.FindInstanceByBusinessKey(w.ID, biz); lookupErr == nil {
		return existing, nil
	}
	i := &model.WorkflowInstance{ID: fmt.Sprintf("inst-%d", time.Now().UnixNano()), WorkflowID: w.ID, Version: 1, WorkflowVersion: w.Version, BizKey: biz, Status: model.Pending, Context: ctx, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := e.s.CreateInstance(i); err != nil {
		return nil, err
	}
	e.s.AppendEvent(model.ExecutionEvent{InstanceID: i.ID, Type: model.EventStarted, Actor: "system"})
	return i, nil
}
func (e *Engine) Advance(ctx context.Context, i *model.WorkflowInstance) error {
	w, err := e.s.GetWorkflow(i.WorkflowID)
	if err != nil {
		return err
	}
	if i.Status == model.Suspended || i.IsTerminal() {
		return nil
	}
	if len(i.CurrentNodeIDs) == 0 {
		n := w.Node("start")
		if n == nil {
			return fmt.Errorf("missing start")
		}
		i.CurrentNodeIDs = []string{n.ID}
	}
	i.Status = model.Running
	for len(i.CurrentNodeIDs) > 0 {
		id := i.CurrentNodeIDs[0]
		i.CurrentNodeIDs = i.CurrentNodeIDs[1:]
		n := w.Node(id)
		if n == nil {
			return fmt.Errorf("node %s", id)
		}
		e.s.AppendEvent(model.NewEvent(i.ID, id, model.EventNodeEntered, "system", nil))
		ni, _ := e.s.GetNode(i.ID, id)
		if ni == nil {
			ni = &model.NodeInstance{ID: fmt.Sprintf("node-%d", time.Now().UnixNano()), InstanceID: i.ID, NodeID: id, NodeType: n.Type, Status: "RUNNING"}
		}
		if ni.Status == "SUCCEEDED" {
			continue
		}
		if ni.Status == "WAITING_RETRY" && n.Type == "wait" {
			ni.MarkSucceeded(time.Now())
			e.s.SaveNode(ni)
			i.Activate(next(w, id, i.Context)...)
			continue
		}
		ni.Attempts++
		switch n.Type {
		case "approval":
			if ni.Status == "WAITING" {
				i.Status = model.WaitingApproval
				e.s.UpdateInstance(i)
				return nil
			}
			t, _ := e.ap.Create(i, *n)
			ni.Status = "WAITING"
			i.Status = model.WaitingApproval
			e.s.SaveNode(ni)
			e.s.AppendEvent(model.NewEvent(i.ID, id, model.EventApprovalCreated, "system", map[string]any{"taskId": t.ID}))
			e.s.UpdateInstance(i)
			return nil
		case "action":
			input := mapInput(n.Input, i.Context)
			out, er := e.a.Run(ctx, *n, input)
			if er != nil {
				ni.LastError = er.Error()
				ni.Status = "FAILED"
				i.Status = model.Failed
				e.s.SaveNode(ni)
				e.s.UpdateInstance(i)
				return er
			}
			applyOutput(n.Output, out, i.Context)
			e.s.AppendEvent(model.NewEvent(i.ID, id, model.EventActionOutput, "system", out))
		case "wait":
			i.Status = model.WaitingRetry
			t := time.Now().Add(time.Second)
			i.NextRetryAt = &t
			e.s.SaveNode(ni)
			e.s.UpdateInstance(i)
			return nil
		case "end":
			i.Status = model.Completed
			now := time.Now()
			i.CompletedAt = &now
		default:
		}
		ni.Status = "SUCCEEDED"
		now := time.Now()
		ni.FinishedAt = now
		e.s.SaveNode(ni)
		i.Activate(next(w, id, i.Context)...)
		e.s.AppendEvent(model.NewEvent(i.ID, id, model.EventNodeCompleted, "system", nil))
	}
	if i.Status == model.Running {
		i.Status = model.Completed
		e.s.AppendEvent(model.ExecutionEvent{InstanceID: i.ID, Type: model.EventCompleted, Actor: "system"})
	}
	return e.s.UpdateInstance(i)
}
func (e *Engine) Approve(id, actor string) error {
	t, err := e.s.GetTask(id)
	if err != nil {
		return err
	}
	if err = e.ap.Approve(id, actor); err != nil {
		return err
	}
	updated, err := e.s.GetTask(id)
	if err != nil {
		return err
	}
	if updated.Status != "APPROVED" {
		return nil
	}
	i, err := e.s.GetInstance(t.InstanceID)
	if err != nil {
		return err
	}
	i.Status = model.Running
	if n, err := e.s.GetNode(t.InstanceID, t.NodeID); err == nil {
		n.Status = "SUCCEEDED"
		n.FinishedAt = time.Now()
		_ = e.s.SaveNode(n)
	}
	w, err := e.s.GetWorkflow(i.WorkflowID)
	if err != nil {
		return err
	}
	i.CurrentNodeIDs = next(w, t.NodeID, i.Context)
	return e.s.UpdateInstance(i)
}

func (e *Engine) Reject(id, actor string) error {
	t, err := e.s.GetTask(id)
	if err != nil {
		return err
	}
	if !t.HasAssignee(actor) {
		return fmt.Errorf("actor is not an assignee")
	}
	if err = e.ap.Reject(id); err != nil {
		return err
	}
	i, err := e.s.GetInstance(t.InstanceID)
	if err != nil {
		return err
	}
	i.Status = model.Failed
	i.ErrorInfo = "approval rejected"
	e.s.AppendEvent(model.NewEvent(i.ID, t.NodeID, model.EventApprovalRejected, actor, nil))
	return e.s.UpdateInstance(i)
}

func mapInput(mapping map[string]string, context map[string]any) map[string]any {
	if len(mapping) == 0 {
		return context
	}
	out := map[string]any{}
	for key, path := range mapping {
		if value, ok := condition.Lookup(path, context); ok {
			out[key] = value
		}
	}
	return out
}
func applyOutput(mapping map[string]string, output, context map[string]any) {
	if context == nil {
		return
	}
	if len(mapping) == 0 {
		for key, value := range output {
			context[key] = value
		}
		return
	}
	for key, path := range mapping {
		if value, ok := condition.Lookup(path, output); ok {
			context[key] = value
		}
	}
}
func (e *Engine) Suspend(id string) error {
	i, err := e.s.GetInstance(id)
	if err != nil {
		return err
	}
	if i.IsTerminal() {
		return fmt.Errorf("terminal instance")
	}
	i.Status = model.Suspended
	e.s.AppendEvent(model.NewEvent(id, "", model.EventInstanceSuspended, "system", nil))
	return e.s.UpdateInstance(i)
}
func (e *Engine) Resume(id string) error {
	i, err := e.s.GetInstance(id)
	if err != nil {
		return err
	}
	if i.Status != model.Suspended {
		return fmt.Errorf("instance is not suspended")
	}
	i.Status = model.Running
	e.s.AppendEvent(model.NewEvent(id, "", model.EventInstanceResumed, "system", nil))
	return e.s.UpdateInstance(i)
}
func (e *Engine) Cancel(id string) error {
	i, err := e.s.GetInstance(id)
	if err != nil {
		return err
	}
	if i.IsTerminal() {
		return fmt.Errorf("terminal instance")
	}
	i.Status = model.Terminated
	now := time.Now()
	i.CompletedAt = &now
	e.s.AppendEvent(model.NewEvent(id, "", model.EventInstanceCancelled, "system", nil))
	return e.s.UpdateInstance(i)
}
