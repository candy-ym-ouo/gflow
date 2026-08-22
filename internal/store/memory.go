package store

import (
	"example.com/gflow/internal/model"
	"sort"
	"sync"
	"time"
)

type Memory struct {
	mu        sync.RWMutex
	workflows map[string]model.WorkflowDefinition
	instances map[string]*model.WorkflowInstance
	nodes     map[string]*model.NodeInstance
	tasks     map[string]*model.ApprovalTask
	events    map[string][]model.ExecutionEvent
	subs      map[int]chan model.ExecutionEvent
	next      int64
	sid       int
}

func NewMemory() *Memory {
	return &Memory{workflows: map[string]model.WorkflowDefinition{}, instances: map[string]*model.WorkflowInstance{}, nodes: map[string]*model.NodeInstance{}, tasks: map[string]*model.ApprovalTask{}, events: map[string][]model.ExecutionEvent{}, subs: map[int]chan model.ExecutionEvent{}}
}
func (m *Memory) SaveWorkflow(d model.WorkflowDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.workflows[d.ID]; ok && existing.Status == "published" && existing.Version == d.Version {
		return ErrConflict
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	d.UpdatedAt = time.Now()
	m.workflows[d.ID] = d
	return nil
}
func (m *Memory) GetWorkflow(id string) (model.WorkflowDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.workflows[id]
	if !ok {
		return d, ErrNotFound
	}
	return d, nil
}
func (m *Memory) ListWorkflows() []model.WorkflowDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := []model.WorkflowDefinition{}
	for _, d := range m.workflows {
		r = append(r, d)
	}
	return r
}
func (m *Memory) CreateInstance(i *model.WorkflowInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.instances {
		if x.WorkflowID == i.WorkflowID && x.BizKey == i.BizKey {
			return ErrDuplicate
		}
	}
	c := cloneInstance(i)
	m.instances[i.ID] = c
	return nil
}
func (m *Memory) GetInstance(id string) (*model.WorkflowInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.instances[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneInstance(i), nil
}
func (m *Memory) UpdateInstance(i *model.WorkflowInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.instances[i.ID]
	if !ok {
		return ErrNotFound
	}
	if i.Version != current.Version {
		return ErrConflict
	}
	i.Version++
	i.UpdatedAt = time.Now()
	m.instances[i.ID] = cloneInstance(i)
	return nil
}

func (m *Memory) ListInstances(f InstanceFilter) []*model.WorkflowInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*model.WorkflowInstance, 0)
	for _, i := range m.instances {
		if f.WorkflowID != "" && i.WorkflowID != f.WorkflowID {
			continue
		}
		if f.Status != "" && i.Status != f.Status {
			continue
		}
		if f.BizKey != "" && i.BizKey != f.BizKey {
			continue
		}
		if !f.IncludeTerminal && i.IsTerminal() {
			continue
		}
		list = append(list, cloneInstance(i))
	}
	sort.Slice(list, func(a, b int) bool { return list[a].CreatedAt.After(list[b].CreatedAt) })
	start := f.Offset
	if start < 0 {
		start = 0
	}
	if start > len(list) {
		start = len(list)
	}
	end := len(list)
	if f.Limit > 0 && start+f.Limit < end {
		end = start + f.Limit
	}
	return list[start:end]
}

func (m *Memory) CountInstances(f InstanceFilter) int {
	f.Limit, f.Offset = 0, 0
	return len(m.ListInstances(f))
}

func (m *Memory) ListDeadLetters() []*model.WorkflowInstance {
	return m.ListInstances(InstanceFilter{Status: model.Dead, IncludeTerminal: true})
}

func (m *Memory) FindInstanceByBusinessKey(workflow, key string) (*model.WorkflowInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, i := range m.instances {
		if i.WorkflowID == workflow && i.BizKey == key {
			return cloneInstance(i), nil
		}
	}
	return nil, ErrNotFound
}
func (m *Memory) ListRunnable(now time.Time) []*model.WorkflowInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := []*model.WorkflowInstance{}
	for _, i := range m.instances {
		if i.Status == model.Pending || i.Status == model.Running || (i.Status == model.WaitingRetry && i.NextRetryAt != nil && !i.NextRetryAt.After(now)) {
			c := *i
			r = append(r, &c)
		}
	}
	return r
}
func (m *Memory) SaveNode(n *model.NodeInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := cloneNode(n)
	m.nodes[n.InstanceID+":"+n.NodeID] = &c
	return nil
}
func (m *Memory) GetNode(i, n string) (*model.NodeInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	x, ok := m.nodes[i+":"+n]
	if !ok {
		return nil, ErrNotFound
	}
	c := cloneNode(x)
	return &c, nil
}
func (m *Memory) ListNodes(i string) []*model.NodeInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := []*model.NodeInstance{}
	for _, n := range m.nodes {
		if n.InstanceID == i {
			c := cloneNode(n)
			r = append(r, &c)
		}
	}
	return r
}
func (m *Memory) SaveTask(t *model.ApprovalTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.tasks[t.ID]; ok && existing.InstanceID != t.InstanceID {
		return ErrDuplicate
	}
	c := cloneTask(t)
	m.tasks[t.ID] = &c
	return nil
}
func (m *Memory) GetTask(id string) (*model.ApprovalTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	c := cloneTask(t)
	return &c, nil
}
func (m *Memory) ListTasks(a string) []*model.ApprovalTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := []*model.ApprovalTask{}
	for _, t := range m.tasks {
		if t.Status == "PENDING" && (a == "" || len(t.Assignees) == 0 || contains(t.Assignees, a)) {
			c := cloneTask(t)
			r = append(r, &c)
		}
	}
	return r
}

func (m *Memory) ListTasksForInstance(id string) []*model.ApprovalTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := make([]*model.ApprovalTask, 0)
	for _, t := range m.tasks {
		if t.InstanceID == id {
			copy := cloneTask(t)
			r = append(r, &copy)
		}
	}
	sort.Slice(r, func(i, j int) bool { return r[i].ID < r[j].ID })
	return r
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func (m *Memory) AppendEvent(e model.ExecutionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	e.Seq = m.next
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	m.events[e.InstanceID] = append(m.events[e.InstanceID], e)
	for _, ch := range m.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
func (m *Memory) Events(id string) []model.ExecutionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.ExecutionEvent{}, m.events[id]...)
}
func (m *Memory) EventsAfter(id string, cursor int64, limit int) []model.ExecutionEvent {
	events := m.Events(id)
	out := make([]model.ExecutionEvent, 0)
	for _, event := range events {
		if event.Seq > cursor {
			out = append(out, event)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
func (m *Memory) Subscribe() (<-chan model.ExecutionEvent, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sid++
	id := m.sid
	ch := make(chan model.ExecutionEvent, 32)
	m.subs[id] = ch
	return ch, func() { m.mu.Lock(); delete(m.subs, id); close(ch); m.mu.Unlock() }
}

func cloneInstance(i *model.WorkflowInstance) *model.WorkflowInstance {
	c := *i
	c.Context = cloneMap(i.Context)
	c.CurrentNodeIDs = append([]string(nil), i.CurrentNodeIDs...)
	if i.NextRetryAt != nil {
		value := *i.NextRetryAt
		c.NextRetryAt = &value
	}
	if i.CompletedAt != nil {
		value := *i.CompletedAt
		c.CompletedAt = &value
	}
	return &c
}
func cloneNode(n *model.NodeInstance) model.NodeInstance {
	c := *n
	c.Input = cloneMap(n.Input)
	c.Output = cloneMap(n.Output)
	if n.NextRetryAt != nil {
		value := *n.NextRetryAt
		c.NextRetryAt = &value
	}
	return c
}
func cloneTask(t *model.ApprovalTask) model.ApprovalTask {
	c := *t
	c.Assignees = append([]string(nil), t.Assignees...)
	if t.Deadline != nil {
		value := *t.Deadline
		c.Deadline = &value
	}
	return c
}
func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
