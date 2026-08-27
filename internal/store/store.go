package store

import (
	"errors"
	"example.com/gflow/internal/model"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("optimistic lock conflict")
	ErrDuplicate = errors.New("duplicate resource")
)

type InstanceFilter struct {
	WorkflowID, Status, BizKey string
	IncludeTerminal            bool
	Limit, Offset              int
}

type Store interface {
	SaveWorkflow(model.WorkflowDefinition) error
	GetWorkflow(string) (model.WorkflowDefinition, error)
	ListWorkflows() []model.WorkflowDefinition
	CreateInstance(*model.WorkflowInstance) error
	GetInstance(string) (*model.WorkflowInstance, error)
	UpdateInstance(*model.WorkflowInstance) error
	ListRunnable(time.Time) []*model.WorkflowInstance
	ListInstances(InstanceFilter) []*model.WorkflowInstance
	CountInstances(InstanceFilter) int
	ListDeadLetters() []*model.WorkflowInstance
	FindInstanceByBusinessKey(string, string) (*model.WorkflowInstance, error)
	SaveNode(*model.NodeInstance) error
	GetNode(string, string) (*model.NodeInstance, error)
	ListNodes(string) []*model.NodeInstance
	SaveTask(*model.ApprovalTask) error
	GetTask(string) (*model.ApprovalTask, error)
	ListTasks(string) []*model.ApprovalTask
	ListTasksForInstance(string) []*model.ApprovalTask
	AppendEvent(model.ExecutionEvent)
	Events(string) []model.ExecutionEvent
	EventsAfter(string, int64, int) []model.ExecutionEvent
	Subscribe() (<-chan model.ExecutionEvent, func())
}
