package validator

import (
	"example.com/gflow/internal/condition"
	"example.com/gflow/internal/model"
	"fmt"
)

type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Validate(d model.WorkflowDefinition) []Issue {
	d = d.Normalize()
	r := []Issue{}
	starts, ends := 0, 0
	seen := map[string]bool{}
	for _, n := range d.Nodes {
		if n.ID == "" || seen[n.ID] {
			r = append(r, Issue{"nodes", "NODE_ID", "node id is required"})
		}
		seen[n.ID] = true
		if n.Type == "start" {
			starts++
		}
		if n.Type == "end" {
			ends++
		}
	}
	if starts != 1 {
		r = append(r, Issue{"nodes", "START_COUNT", fmt.Sprintf("want one start, got %d", starts)})
	}
	if ends != 1 {
		r = append(r, Issue{"nodes", "END_COUNT", fmt.Sprintf("want one end, got %d", ends)})
	}
	for _, e := range d.Edges {
		if !seen[e.From] || !seen[e.To] {
			r = append(r, Issue{"edges", "EDGE_DANGLING", "edge references unknown node"})
		}
	}
	for index, n := range d.Nodes {
		if !model.NodeTypes[n.Type] {
			r = append(r, Issue{fmt.Sprintf("nodes[%d].type", index), "NODE_TYPE", "unsupported node type"})
		}
		out := d.Out(n.ID)
		in := d.In(n.ID)
		if n.Type == "start" && len(in) > 0 {
			r = append(r, Issue{"nodes", "START_IN_EDGE", "start cannot have incoming edges"})
		}
		if n.Type == "end" && len(out) > 0 {
			r = append(r, Issue{"nodes", "END_OUT_EDGE", "end cannot have outgoing edges"})
		}
		if n.Type == "condition" {
			r = append(r, validateConditionEdges(n.ID, out, d, index)...)
		}
		if n.Type == "approval" {
			r = append(r, validateApproval(n, index)...)
		}
		if n.Type == "action" {
			r = append(r, validateAction(n, index)...)
		}
		if n.Type == "wait" {
			if n.Config == nil || (n.Config["duration"] == nil && n.Config["until"] == nil) {
				r = append(r, Issue{fmt.Sprintf("nodes[%d]", index), "WAIT_CONFIG", "wait requires duration or until"})
			}
		}
	}
	if reachable := d.Reachable(); len(reachable) != len(d.Nodes) {
		r = append(r, Issue{"nodes", "UNREACHABLE", "all nodes must be reachable from start"})
	}
	r = append(r, validateCycles(d)...)
	return r
}

func validateConditionEdges(id string, edges []model.Edge, d model.WorkflowDefinition, index int) []Issue {
	r := []Issue{}
	if len(edges) < 2 {
		r = append(r, Issue{fmt.Sprintf("nodes[%d].edges", index), "CONDITION_EDGES", "condition needs at least two outgoing edges"})
	}
	defaults := 0
	orders := map[int]bool{}
	for _, e := range edges {
		if e.Default {
			defaults++
		}
		if orders[e.Order] && e.Order != 0 {
			r = append(r, Issue{"edges", "EDGE_ORDER", "edge order must be unique"})
		}
		orders[e.Order] = true
		if e.Condition != "" {
			if _, err := condition.Eval(e.Condition, map[string]any{}); err != nil {
				r = append(r, Issue{"edges.condition", "CONDITION_SYNTAX", err.Error()})
			}
		}
	}
	if defaults != 1 {
		r = append(r, Issue{fmt.Sprintf("nodes[%d].edges", index), "DEFAULT_EDGE", "condition needs exactly one default edge"})
	}
	return r
}

func validateApproval(n model.Node, index int) []Issue {
	r := []Issue{}
	mode := ""
	if n.Config != nil {
		mode, _ = n.Config["mode"].(string)
	}
	if mode == "" {
		mode = "single"
	}
	valid := map[string]bool{"single": true, "countersign": true, "any": true, "sequence": true}
	if !valid[mode] {
		r = append(r, Issue{fmt.Sprintf("nodes[%d].config.mode", index), "APPROVAL_MODE", "unsupported approval mode"})
	}
	if n.Config == nil || (n.Config["assignee"] == nil && n.Config["assignees"] == nil) {
		r = append(r, Issue{fmt.Sprintf("nodes[%d].config", index), "APPROVAL_ASSIGNEE", "approval requires assignee or assignees"})
	}
	return r
}

func validateAction(n model.Node, index int) []Issue {
	if n.Config == nil {
		return []Issue{{fmt.Sprintf("nodes[%d].config", index), "ACTION_CONFIG", "action requires config"}}
	}
	kind, _ := n.Config["kind"].(string)
	valid := map[string]bool{"http": true, "notify": true, "wait": true, "function": true, "fail": true}
	if !valid[kind] {
		return []Issue{{fmt.Sprintf("nodes[%d].config.kind", index), "ACTION_KIND", "unsupported action kind"}}
	}
	return nil
}

func validateCycles(d model.WorkflowDefinition) []Issue {
	state := map[string]int{}
	result := []Issue{}
	var visit func(string)
	visit = func(id string) {
		state[id] = 1
		for _, edge := range d.Out(id) {
			if state[edge.To] == 1 && edge.Condition == "" {
				result = append(result, Issue{"edges", "UNCONDITIONAL_CYCLE", "unconditional cycle detected"})
			}
			if state[edge.To] == 0 {
				visit(edge.To)
			}
		}
		state[id] = 2
	}
	if start := d.Start(); start != nil {
		visit(start.ID)
	}
	return result
}

func ErrorsByCode(issues []Issue) map[string]int {
	out := map[string]int{}
	for _, issue := range issues {
		out[issue.Code]++
	}
	return out
}
