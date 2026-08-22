package model

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID, Name, Type string
	Config         map[string]any
	Input, Output  map[string]string
	Retry          *RetryPolicy
}
type Edge struct {
	From, To, Condition string
	Order               int
	Default             bool
}
type RetryPolicy struct {
	MaxAttempts int `json:"maxAttempts"`
	Interval    string
	Backoff     string
	MaxInterval string
}
type RecoveryConfig struct {
	InstanceMaxRetry int  `json:"instanceMaxRetry"`
	DeadLetter       bool `json:"deadLetter"`
}
type WorkflowDefinition struct {
	ID, Name             string
	Version              int
	Status               string
	Nodes                []Node
	Edges                []Edge
	Recovery             RecoveryConfig
	CreatedAt, UpdatedAt time.Time
}

func (d WorkflowDefinition) Node(id string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].ID == id {
			return &d.Nodes[i]
		}
	}
	return nil
}
func (d WorkflowDefinition) Out(id string) []Edge {
	r := []Edge{}
	for _, e := range d.Edges {
		if e.From == id {
			r = append(r, e)
		}
	}
	return r
}

var NodeTypes = map[string]bool{"start": true, "end": true, "approval": true, "action": true, "condition": true, "parallel": true, "join": true, "wait": true}

func (d WorkflowDefinition) In(id string) []Edge {
	r := make([]Edge, 0)
	for _, e := range d.Edges {
		if e.To == id {
			r = append(r, e)
		}
	}
	return r
}

func (d WorkflowDefinition) Start() *Node {
	for i := range d.Nodes {
		if d.Nodes[i].Type == "start" {
			return &d.Nodes[i]
		}
	}
	return nil
}

func (d WorkflowDefinition) End() *Node {
	for i := range d.Nodes {
		if d.Nodes[i].Type == "end" {
			return &d.Nodes[i]
		}
	}
	return nil
}

func (d WorkflowDefinition) SortedOut(id string) []Edge {
	r := d.Out(id)
	sort.SliceStable(r, func(i, j int) bool { return r[i].Order < r[j].Order })
	return r
}

func (d WorkflowDefinition) Reachable() map[string]bool {
	seen := map[string]bool{}
	start := d.Start()
	if start == nil {
		return seen
	}
	queue := []string{start.ID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		for _, e := range d.Out(id) {
			if !seen[e.To] {
				queue = append(queue, e.To)
			}
		}
	}
	return seen
}

func (d WorkflowDefinition) Clone() WorkflowDefinition {
	b, _ := json.Marshal(d)
	var out WorkflowDefinition
	_ = json.Unmarshal(b, &out)
	return out
}

func (n Node) Kind() string {
	if n.Config == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(toString(n.Config["kind"])))
}

func (n Node) RetryPolicy() RetryPolicy {
	if n.Retry == nil {
		return RetryPolicy{MaxAttempts: 1, Interval: "1s", Backoff: "fixed"}
	}
	p := *n.Retry
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}
	if p.Interval == "" {
		p.Interval = "1s"
	}
	if p.Backoff == "" {
		p.Backoff = "fixed"
	}
	return p
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (d WorkflowDefinition) Normalize() WorkflowDefinition {
	out := d.Clone()
	if out.Version < 1 {
		out.Version = 1
	}
	if out.Status == "" {
		out.Status = "draft"
	}
	for i := range out.Nodes {
		if out.Nodes[i].Name == "" {
			out.Nodes[i].Name = out.Nodes[i].ID
		}
		if out.Nodes[i].Config == nil {
			out.Nodes[i].Config = map[string]any{}
		}
	}
	return out
}
