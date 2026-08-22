package model

import "time"

const (
	Pending         = "PENDING"
	Running         = "RUNNING"
	WaitingApproval = "WAITING_APPROVAL"
	WaitingRetry    = "WAITING_RETRY"
	Completed       = "COMPLETED"
	Failed          = "FAILED"
	Dead            = "DEAD"
	Suspended       = "SUSPENDED"
	Terminated      = "TERMINATED"
)

type WorkflowInstance struct {
	ID, WorkflowID, BizKey, Status string
	WorkflowVersion                int
	Context                        map[string]any
	CurrentNodeIDs                 []string
	Version                        int
	ErrorInfo                      string
	NextRetryAt                    *time.Time
	CreatedAt, UpdatedAt           time.Time
	CompletedAt                    *time.Time
}
type NodeInstance struct {
	ID, InstanceID, NodeID, NodeType, Status, LastError string
	Attempts                                            int
	Input, Output                                       map[string]any
	NextRetryAt                                         *time.Time
	StartedAt, FinishedAt                               time.Time
}
type ApprovalTask struct {
	ID, InstanceID, NodeID, Mode, Status string
	Assignees                            []string
	PassRatio                            float64
	PassCount                            int
	Deadline                             *time.Time
	TimeoutPolicy                        string
}

func (i *WorkflowInstance) IsTerminal() bool {
	return i.Status == Completed || i.Status == Failed || i.Status == Dead || i.Status == Terminated
}

func (i *WorkflowInstance) CanAdvance() bool {
	return i.Status == Pending || i.Status == Running || i.Status == WaitingRetry
}

func (i *WorkflowInstance) SetContext(key string, value any) {
	if i.Context == nil {
		i.Context = map[string]any{}
	}
	i.Context[key] = value
}

func (i *WorkflowInstance) ContextValue(key string) (any, bool) {
	if i.Context == nil {
		return nil, false
	}
	v, ok := i.Context[key]
	return v, ok
}

func (i *WorkflowInstance) Activate(ids ...string) {
	for _, id := range ids {
		found := false
		for _, current := range i.CurrentNodeIDs {
			if current == id {
				found = true
				break
			}
		}
		if !found && id != "" {
			i.CurrentNodeIDs = append(i.CurrentNodeIDs, id)
		}
	}
}

func (i *WorkflowInstance) Deactivate(id string) {
	filtered := i.CurrentNodeIDs[:0]
	for _, current := range i.CurrentNodeIDs {
		if current != id {
			filtered = append(filtered, current)
		}
	}
	i.CurrentNodeIDs = filtered
}

func (n *NodeInstance) MarkRunning(now time.Time) {
	n.Status = "RUNNING"
	n.Attempts++
	if n.StartedAt.IsZero() {
		n.StartedAt = now
	}
}
func (n *NodeInstance) MarkSucceeded(now time.Time) { n.Status = "SUCCEEDED"; n.FinishedAt = now }
func (n *NodeInstance) MarkFailed(err error) {
	n.Status = "FAILED"
	if err != nil {
		n.LastError = err.Error()
	}
}
func (n *NodeInstance) Ready(now time.Time) bool {
	return n.Status == "READY" || (n.Status == "WAITING_RETRY" && n.NextRetryAt != nil && !n.NextRetryAt.After(now))
}

func (t *ApprovalTask) IsPending() bool { return t.Status == "PENDING" }
func (t *ApprovalTask) HasAssignee(actor string) bool {
	for _, a := range t.Assignees {
		if a == actor {
			return true
		}
	}
	return len(t.Assignees) == 0
}
