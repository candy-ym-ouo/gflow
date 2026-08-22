package engine

import (
	"example.com/gflow/internal/condition"
	"example.com/gflow/internal/model"
)

func next(d model.WorkflowDefinition, id string, ctx map[string]any) []string {
	r := []string{}
	for _, e := range d.Out(id) {
		ok, _ := condition.Eval(e.Condition, ctx)
		if ok || e.Default {
			r = append(r, e.To)
		}
	}
	return r
}
