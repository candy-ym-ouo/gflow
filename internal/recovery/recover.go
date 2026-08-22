package recovery

import (
	"example.com/gflow/internal/model"
	"fmt"
	"time"
)

type Decision struct {
	Status     string
	RetryAt    *time.Time
	DeadLetter bool
	Reason     string
}

func Decide(policy Policy, attempts int, err error, deadLetter bool) Decision {
	if err == nil {
		return Decision{Status: "success"}
	}
	if Classify(err) == Fatal || policy.Exhausted(attempts) {
		status := model.Failed
		if deadLetter {
			status = model.Dead
		}
		return Decision{Status: status, DeadLetter: deadLetter, Reason: err.Error()}
	}
	next := policy.Next(attempts)
	return Decision{Status: model.WaitingRetry, RetryAt: &next, Reason: err.Error()}
}
func RetryInstance(i *model.WorkflowInstance) error {
	if i.Status != model.Dead && i.Status != model.Failed {
		return fmt.Errorf("instance %s is not retryable", i.ID)
	}
	i.Status = model.Running
	i.ErrorInfo = ""
	i.NextRetryAt = nil
	return nil
}
func Suspend(i *model.WorkflowInstance) error {
	if i.IsTerminal() {
		return fmt.Errorf("instance %s is terminal", i.ID)
	}
	i.Status = model.Suspended
	return nil
}
func Resume(i *model.WorkflowInstance) error {
	if i.Status != model.Suspended {
		return fmt.Errorf("instance %s is not suspended", i.ID)
	}
	i.Status = model.Running
	return nil
}
func Cancel(i *model.WorkflowInstance) error {
	if i.IsTerminal() {
		return fmt.Errorf("instance %s is terminal", i.ID)
	}
	i.Status = model.Terminated
	now := time.Now()
	i.CompletedAt = &now
	return nil
}
