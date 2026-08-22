package model

import "time"

type ExecutionEvent struct {
	Seq                             int64 `json:"seq"`
	InstanceID, NodeID, Type, Actor string
	Detail                          map[string]any `json:"detail,omitempty"`
	OccurredAt                      time.Time      `json:"occurredAt"`
}

const (
	EventStarted         = "INSTANCE_STARTED"
	EventNodeEntered     = "NODE_ENTERED"
	EventNodeCompleted   = "NODE_COMPLETED"
	EventApprovalCreated = "APPROVAL_CREATED"
	EventApprovalDone    = "APPROVAL_DONE"
	EventCompleted       = "COMPLETED"
	EventFailed          = "INSTANCE_FAILED"
)

const (
	EventRetryScheduled    = "RETRY_SCHEDULED"
	EventDeadLettered      = "DEAD_LETTERED"
	EventInstanceSuspended = "INSTANCE_SUSPENDED"
	EventInstanceResumed   = "INSTANCE_RESUMED"
	EventInstanceCancelled = "INSTANCE_CANCELLED"
	EventApprovalRejected  = "APPROVAL_REJECTED"
	EventActionOutput      = "ACTION_OUTPUT"
)

func NewEvent(instance, node, typ, actor string, detail map[string]any) ExecutionEvent {
	return ExecutionEvent{InstanceID: instance, NodeID: node, Type: typ, Actor: actor, Detail: detail, OccurredAt: time.Now()}
}

func (e ExecutionEvent) HasDetail(key string) bool { _, ok := e.Detail[key]; return ok }
